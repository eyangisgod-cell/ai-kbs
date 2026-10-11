// Package observe is the AI-KBS HTTP-spans facade over the embedded
// tracing/langfuse OTel client. It exposes the env-var contract required
// by design.md §4.5 and prd.md §3.1 AC-B8:
//
//   - Business span → OTLP → Langfuse (this package; T1.8).
//   - Aggregate infra metrics → Prometheus → Grafana (separate package; T1.21).
//
// The two channels do not overlap: OTLP carries per-request trace_id,
// Prometheus carries aggregate counters/histograms. They cross-link via
// the trace_id and a foreign-key join in the Langfuse / Grafana UI.
//
// This file defines the OTLP configuration. We deliberately use a
// dedicated env-var namespace (OTLP_*) instead of LANGFUSE_* because
// design.md §4.5 names the OTLP protocol as the integration contract
// (Langfuse is one OTLP-compatible backend among many: Tempo, Honeycomb,
// OTel Collector, ...). A future operator can point the same config at a
// self-hosted OTel collector without renaming variables.
package observe

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration for the OTLP/Langfuse client.
// All fields are sourced from environment variables via LoadConfig — the
// struct is intentionally exposed so other packages (the bootstrap layer
// in cmd/server, the CLI's `ai-kbs status` output) can introspect the
// resolved values without re-implementing env-var parsing.
//
// In practice users enable observability purely through environment
// variables — Endpoint / PublicKey / SecretKey — which matches every
// other OTel/OTLP client and keeps the YAML config free of secrets.
type Config struct {
	// Enabled is the master switch. When false the package is a no-op
	// (middleware is pass-through, helpers ignore the manager).
	Enabled bool
	// Endpoint is the OTLP/HTTP base URL, e.g.
	//   https://cloud.langfuse.com           (managed Langfuse)
	//   https://<region>.cloud.langfuse.com  (managed regional)
	//   https://otel.example.com              (self-hosted collector / Tempo / ...)
	// The path "/api/public/otel/v1/traces" is appended automatically by
	// the language-specific protocol guides — callers should NOT add it.
	Endpoint string
	// PublicKey / SecretKey are project credentials (sent as HTTP Basic
	// for Langfuse) OR left empty when the backend is an unauthenticated
	// collector.
	PublicKey string
	SecretKey string
	// Release / Environment are attached to every trace for filtering in
	// the trace UI (e.g. release="v0.4.2", environment="production").
	Release     string
	Environment string
	// FlushAt: OTLP batch export — flush once the queue reaches this size.
	FlushAt int
	// FlushInterval: maximum time between automatic flushes.
	FlushInterval time.Duration
	// QueueSize bounds the in-memory buffer to avoid unbounded growth if
	// the OTLP endpoint is unreachable.
	QueueSize int
	// RequestTimeout: HTTP timeout for a single ingestion batch.
	RequestTimeout time.Duration
	// SampleRate (0..1) controls trace sampling. 0 means "use 1.0".
	SampleRate float64
}

// LoadConfig builds a Config by reading the OTLP_* environment
// variables. Variable names are part of the public contract (operators
// script them); the parsing rules below are pinned by config_test.go so
// any rename fails loudly in CI.
//
// Env variables (all optional — absent means disabled):
//
//	OTLP_ENABLED                "1"/"true"/"yes"/"on"  force-enable
//	OTLP_ENDPOINT               "https://..."          OTLP/HTTP base URL
//	OTLP_LANGFUSE_PUBLIC_KEY    "pk-..."               Langfuse project public key
//	OTLP_LANGFUSE_SECRET_KEY    "sk-..."               Langfuse project secret key
//	OTLP_RELEASE                "v0.4.2"               trace attribute
//	OTLP_ENVIRONMENT            "production"           trace attribute
//	OTLP_FLUSH_AT               "15"                   integer > 0
//	OTLP_FLUSH_INTERVAL         "3s" or "3"            duration or integer seconds
//	OTLP_QUEUE_SIZE             "2048"                 integer > 0
//	OTLP_REQUEST_TIMEOUT        "10s"                  duration only
//	OTLP_SAMPLE_RATE            "0.25"                 float in [0, 1]
//
// Special case: when OTLP_ENDPOINT is set AND both
// OTLP_LANGFUSE_PUBLIC_KEY and OTLP_LANGFUSE_SECRET_KEY are set,
// Enabled auto-flips to true (matching the convention from
// tracing/langfuse — operators should only need to set credentials).
func LoadConfig() Config {
	cfg := Config{
		Endpoint:       strings.TrimSpace(os.Getenv("OTLP_ENDPOINT")),
		PublicKey:      strings.TrimSpace(os.Getenv("OTLP_LANGFUSE_PUBLIC_KEY")),
		SecretKey:      strings.TrimSpace(os.Getenv("OTLP_LANGFUSE_SECRET_KEY")),
		Release:        strings.TrimSpace(os.Getenv("OTLP_RELEASE")),
		Environment:    strings.TrimSpace(os.Getenv("OTLP_ENVIRONMENT")),
		FlushAt:        15,
		FlushInterval:  3 * time.Second,
		QueueSize:      2048,
		RequestTimeout: 10 * time.Second,
		SampleRate:     1.0,
	}

	// Explicit OTLP_ENABLED always wins; absent falls back to "auto-on
	// when (endpoint + credentials) are present".
	if v := strings.TrimSpace(os.Getenv("OTLP_ENABLED")); v != "" {
		cfg.Enabled = parseBool(v)
	} else if cfg.Endpoint != "" && cfg.PublicKey != "" && cfg.SecretKey != "" {
		cfg.Enabled = true
	}

	if v := strings.TrimSpace(os.Getenv("OTLP_FLUSH_AT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.FlushAt = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("OTLP_FLUSH_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.FlushInterval = d
		} else if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.FlushInterval = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("OTLP_QUEUE_SIZE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.QueueSize = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("OTLP_REQUEST_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.RequestTimeout = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("OTLP_SAMPLE_RATE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.SampleRate = f
		}
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}

	return cfg
}

// Validate verifies required fields are present when observability is
// enabled. Disabled configs are trivially valid so the bootstrap layer
// can call Validate() unconditionally.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("observe: otlp_endpoint is required when enabled")
	}
	if c.PublicKey == "" || c.SecretKey == "" {
		return fmt.Errorf("observe: otlp_langfuse_public_key and otlp_langfuse_secret_key are required when enabled")
	}
	return nil
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	}
	return false
}
