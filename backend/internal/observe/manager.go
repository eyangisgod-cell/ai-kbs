package observe

import (
	"context"
	"sync"

	"github.com/eyangisgod-cell/ai-kbs/internal/logger"
	"github.com/eyangisgod-cell/ai-kbs/internal/tracing/langfuse"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Manager is the AI-KBS-facing facade over the langfuse package. We do
// not re-implement OTel here — the langfuse.Manager already owns an
// OTel TracerProvider backed by an OTLP/HTTP exporter. This type exists
// so observe can:
//
//   (a) own the env-var-driven configuration (LoadConfig),
//   (b) expose Enabled() / Tracer() / Shutdown() compatible with every
//       existing call site in the OTel ecosystem,
//   (c) sit behind a package-wide singleton (Manager()) without leaking
//       the langfuse.Manager into every importing package — so a future
//       swap to a different OTLP backend (Tempo, Honeycomb, a bare OTel
//       collector) is a one-file change.
//
// When Langfuse initialization fails (e.g. unset PublicKey), the
// returned Manager is a permanent no-op so Enabled() always returns
// false and middleware degrades to pass-through; no panic.
type Manager struct {
	enabled   bool
	tracer    trace.Tracer
	lfManager *langfuse.Manager
	mu        sync.RWMutex
	closed    bool
}

var (
	globalMu sync.RWMutex
	global   *Manager
)

// Init configures and installs the package-wide Manager singleton.
// Returns a non-nil *Manager even on error / disabled — the manager is
// always safe to defer against. The langfuse Manager is constructed
// under the hood using the same LANGFUSE_* env vars it already reads;
// observe's LoadConfig returns the mirror-shape Config so the bootstrap
// layer can log the resolved values without re-parsing.
//
// Semantics:
//   - When cfg.Enabled is false, returns a no-op manager (Enabled=false)
//     so the middleware downgrades cleanly without an "init failed"
//     log line — disabled is the default state, not an error.
//   - When cfg.Enabled is true but langfuse.Init fails (bad env), the
//     error is returned AND a no-op manager is installed, so subsequent
//     calls don't panic on a nil pointer.
func Init(cfg Config) (*Manager, error) {
	// Translate the observe-side Config into the langfuse-side Config
	// shape. OTLP_ENDPOINT maps to langfuse's Host (Langfuse v3+ exposes
	// its OTel endpoint at <host>/api/public/otel/v1/traces).
	lfCfg := langfuse.Config{
		Enabled:        cfg.Enabled,
		Host:           cfg.Endpoint,
		PublicKey:      cfg.PublicKey,
		SecretKey:      cfg.SecretKey,
		Release:        cfg.Release,
		Environment:    cfg.Environment,
		FlushAt:        cfg.FlushAt,
		FlushInterval:  cfg.FlushInterval,
		QueueSize:      cfg.QueueSize,
		RequestTimeout: cfg.RequestTimeout,
		SampleRate:     cfg.SampleRate,
	}

	if !cfg.Enabled {
		mgr := &Manager{enabled: false}
		globalMu.Lock()
		global = mgr
		globalMu.Unlock()
		logger.Infof(context.Background(),
			"[observe] otlp disabled (no OTLP_* env vars set); middleware is pass-through")
		return mgr, nil
	}

	if err := cfg.Validate(); err != nil {
		// Preserve fail-closed semantics: a misconfigured enabled path
		// MUST NOT silently start emitting spans to an unauthorised or
		// unreachable endpoint.
		mgr := &Manager{enabled: false}
		globalMu.Lock()
		global = mgr
		globalMu.Unlock()
		return mgr, err
	}

	lfMgr, err := langfuse.Init(lfCfg)
	if err != nil {
		mgr := &Manager{enabled: false}
		globalMu.Lock()
		global = mgr
		globalMu.Unlock()
		return mgr, err
	}

	mgr := &Manager{
		enabled:   true,
		tracer:    lfMgr.Tracer(),
		lfManager: lfMgr,
	}
	globalMu.Lock()
	global = mgr
	globalMu.Unlock()
	logger.Infof(context.Background(),
		"[observe] otlp enabled endpoint=%s public_key=%s (OTLP/HTTP via Langfuse v3+)",
		cfg.Endpoint, cfg.PublicKey)
	return mgr, nil
}

// GetManager returns the installed singleton, or nil if Init has not
// been called. Callers should treat a nil return as "disabled" and skip
// emitting spans. To make the common case ergonomic, the RootSpan
// middleware also tolerates a nil receiver.
func GetManager() *Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Enabled reports whether this manager would emit spans. Always returns
// false on a nil receiver.
func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled && !m.closed && m.tracer != nil
}

// Tracer returns the OTel tracer so middleware can open root spans
// directly. The tracer is non-nil when Enabled() is true. When disabled,
// returns the noop tracer so callers don't need a nil check.
func (m *Manager) Tracer() trace.Tracer {
	if !m.Enabled() {
		return otel.GetTracerProvider().Tracer(otelTracerName)
	}
	return m.tracer
}

// Shutdown flushes pending spans and releases the exporter. Safe to
// call multiple times.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.lfManager == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.mu.Unlock()
	return m.lfManager.Shutdown(ctx)
}
