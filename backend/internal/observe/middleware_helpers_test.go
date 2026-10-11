package observe

import (
	"context"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// spanRecorder wraps an in-memory SpanExporter + a Manager wired to it
// for testing the root-span middleware.
//
// Use:
//
//	rec := newSpanRecorder(t)
//	t.Cleanup(rec.shutdown)
//	// pass rec.manager into RootSpanMiddleware
//	id := rec.rootTraceID() // read after the middleware has run
type spanRecorder struct {
	manager *Manager
	exp     *memoryExporter
}

// newSpanRecorder builds a fresh in-memory span recorder for a test.
// Each call returns an independent exporter/manager pair so tests can
// run in parallel without cross-contamination.
//
// We bypass the production Init path (which tries to connect to the
// Langfuse endpoint) and instead wire a SimpleSpanProcessor pointing at
// our in-memory exporter. The Manager struct still owns the same
// observable surface (Enabled, Tracer, Shutdown) so middleware
// behaviour is identical.
func newSpanRecorder(t *testing.T) *spanRecorder {
	t.Helper()
	exp := newMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)),
	)
	tracer := tp.Tracer("observe-test")
	mgr := &Manager{enabled: true, tracer: tracer}
	return &spanRecorder{manager: mgr, exp: exp}
}

// shutdown releases the manager. Idempotent.
func (r *spanRecorder) shutdown() {
	if r == nil || r.manager == nil {
		return
	}
	_ = r.manager.Shutdown(context.Background())
	r.manager.mu.Lock()
	if !r.manager.closed {
		r.manager.closed = true
	}
	r.manager.mu.Unlock()
}

// rootTraceID returns the trace id of the most recent server-kind root
// span recorded by the exporter. Returns "" if none.
func (r *spanRecorder) rootTraceID() string {
	if r == nil || r.exp == nil {
		return ""
	}
	r.exp.mu.Lock()
	defer r.exp.mu.Unlock()
	for i := len(r.exp.spans) - 1; i >= 0; i-- {
		s := r.exp.spans[i]
		if s.SpanKind == oteltrace.SpanKindServer {
			return s.SpanContext.TraceID().String()
		}
	}
	return ""
}

// allSpans returns a snapshot of every span recorded by the exporter so
// far, in emission order. Caller MUST NOT mutate the returned slice.
func (r *spanRecorder) allSpans() []recordedSpan {
	if r == nil || r.exp == nil {
		return nil
	}
	r.exp.mu.Lock()
	defer r.exp.mu.Unlock()
	out := make([]recordedSpan, len(r.exp.spans))
	copy(out, r.exp.spans)
	return out
}

// recordedSpan is the snapshot we keep per exported span. We only
// retain fields our tests read to keep memory bounded.
type recordedSpan struct {
	Name         string
	SpanKind     oteltrace.SpanKind
	SpanContext  oteltrace.SpanContext
	ParentSpanID oteltrace.SpanID
}

// memoryExporter is a minimal sdktrace.SpanExporter that records spans
// in-memory for assertions in tests. Implements just enough of the SDK
// interface for a SimpleSpanProcessor to call Export + Shutdown.
//
// Tests use it via a SimpleSpanProcessor so spans flush synchronously on
// span.End — deterministic, no flakiness from polling.
type memoryExporter struct {
	mu    sync.Mutex
	spans []recordedSpan
}

func newMemoryExporter() *memoryExporter {
	return &memoryExporter{}
}

// ExportSpans is the SpanExporter interface method.
func (m *memoryExporter) ExportSpans(_ context.Context, ss []sdktrace.ReadOnlySpan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range ss {
		m.spans = append(m.spans, recordedSpan{
			Name:         s.Name(),
			SpanKind:     s.SpanKind(),
			SpanContext:  s.SpanContext(),
			ParentSpanID: s.Parent().SpanID(),
		})
	}
	return nil
}

// Shutdown is the SpanExporter interface method (no-op).
func (m *memoryExporter) Shutdown(context.Context) error { return nil }
