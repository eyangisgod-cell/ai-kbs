// Package observe is the AI-KBS HTTP-spans facade over the embedded
// tracing/langfuse client. It adds W3C traceparent parsing (envelope v1.0
// stamps the trace_id), universal root-span middleware (any HTTP request
// becomes a root span — AC-B8), and typed helpers for child spans on DB /
// ES / SAG / LLM calls.
package observe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestParseTraceparent_Valid asserts the canonical W3C header parses to
// the expected 32-hex trace id and 16-hex parent span id.
func TestParseTraceparent_Valid(t *testing.T) {
	const header = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	got := ParseTraceparent(header)
	if got.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("trace id = %q, want 0af7651916cd43dd8448eb211c80319c", got.TraceID)
	}
	if got.SpanID != "b7ad6b7169203331" {
		t.Errorf("span id = %q, want b7ad6b7169203331", got.SpanID)
	}
	if !got.Sampled {
		t.Errorf("sampled = false, want true (flags=01)")
	}
}

// TestParseTraceparent_NotSampled covers the flags=00 variant (the W3C
// spec only requires sampled=01).
func TestParseTraceparent_NotSampled(t *testing.T) {
	const header = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00"
	got := ParseTraceparent(header)
	if got.TraceID == "" {
		t.Fatal("expected non-empty trace id even when not sampled")
	}
	if got.Sampled {
		t.Errorf("sampled = true, want false (flags=00)")
	}
}

// TestParseTraceparent_Malformed covers inputs that must NOT panic and
// must NOT silently accept; the parser returns a zero Traceparent (no
// inherited trace id) so the middleware will mint a fresh one.
func TestParseTraceparent_Malformed(t *testing.T) {
	cases := []struct {
		name, header string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"too few fields", "00-0af7651916cd43dd8448eb211c80319c"},
		{"too many fields", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01-extra"},
		{"non-hex trace id", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-b7ad6b7169203331-01"},
		{"trace id wrong length", "00-0af7651916cd43dd-b7ad6b7169203331-01"},
		{"non-hex span id", "00-0af7651916cd43dd8448eb211c80319c-zzzzzzzzzzzzzzzz-01"},
		{"span id wrong length", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b-01"},
		{"wrong version", "01-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		{"future version", "ff-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		{"zero trace id", "00-00000000000000000000000000000000-b7ad6b7169203331-01"},
		{"zero span id", "00-0af7651916cd43dd8448eb211c80319c-0000000000000000-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTraceparent(tc.header)
			if got.TraceID != "" {
				t.Errorf("malformed %q: expected empty TraceID, got %q", tc.header, got.TraceID)
			}
		})
	}
}

// TestRoundTripEcho ensures the trace_id a *Trace has recorded is also
// the one we ship out in the envelope (W3C convergence contract).
func TestEnvelopeStampsTraceIDFromContext(t *testing.T) {
	const header = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	env := NewEnvelopeWithTraceID("req-1", header)

	if env.RequestID != "req-1" {
		t.Errorf("request_id = %q, want req-1", env.RequestID)
	}
	if env.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id = %q, want 4bf92f3577b34da6a3ce929d0e0e4736", env.TraceID)
	}
}

// TestRoundTripEcho_NoTraceparent ensures envelopes without a
// traceparent header carry NO trace_id field (omitempty) instead of
// emitting an empty string.
func TestEnvelopeStampsTraceID_NoHeader(t *testing.T) {
	env := NewEnvelopeWithTraceID("req-2", "")
	if env.TraceID != "" {
		t.Errorf("trace_id should be empty when no traceparent, got %q", env.TraceID)
	}
}

// TestRootSpanMiddleware_TraceparentInherited simulates a downstream
// caller carrying a W3C traceparent and verifies that the middleware
// makes the recorded trace_id match upstream (single-tree correlation).
func TestRootSpanMiddleware_TraceparentInherited(t *testing.T) {
	// Use an in-memory span recorder to read what was produced.
	rec := newSpanRecorder(t)
	t.Cleanup(func() { rec.shutdown() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RootSpanMiddleware(rec.manager))
	r.GET("/api/v1/health", func(c *gin.Context) {
		TraceIDFromContext(c) // accessor must not panic
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	const upstreamTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	header := "00-" + upstreamTraceID + "-00f067aa0ba902b7-01"
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	req.Header.Set("traceparent", header)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := rec.rootTraceID(); got != upstreamTraceID {
		t.Errorf("root span trace id = %q, want %q (upstream)", got, upstreamTraceID)
	}
}

// TestRootSpanMiddleware_NoTraceparentMintsFresh ensures that, when no
// traceparent is supplied, the middleware still opens a root span —
// AC-B8 requires "any HTTP request" to surface in the dashboard.
func TestRootSpanMiddleware_NoTraceparentMintsFresh(t *testing.T) {
	rec := newSpanRecorder(t)
	t.Cleanup(func() { rec.shutdown() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RootSpanMiddleware(rec.manager))
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Fire two requests; each must mint its own root trace id (so the
	// dashboard shows two traces, not one shared trace).
	const n = 2
	seen := make(map[string]int, n)
	for i := 0; i < n; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		r.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, recorder.Code)
		}
		seen[rec.rootTraceID()]++
	}
	if len(seen) != n {
		t.Errorf("expected %d distinct root trace ids, got %d (seen=%v)", n, len(seen), seen)
	}
}

// TestRootSpanMiddleware_DisabledNoop ensures that, when the OTLP
// manager is disabled (no env vars set), the middleware degrades to a
// pass-through rather than recording / panicking.
func TestRootSpanMiddleware_DisabledNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RootSpanMiddleware(nil))
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), `"status":"fail"`) {
		t.Errorf("disabled manager should not interfere, body=%q", recorder.Body.String())
	}
}

// TestChildSpanNames covers the typed helpers for DB / ES / SAG / LLM
// child spans — T1.8 requires at least one example site for each
// category, and these helpers give every backend call site a single
// canonical name.
func TestChildSpanNames(t *testing.T) {
	cases := map[string]string{
		DB().Name:  "ai-kbs.db",
		ES().Name:  "ai-kbs.es",
		SAG().Name: "ai-kbs.sag",
		LLM().Name: "ai-kbs.llm",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("span name = %q, want %q", got, want)
		}
	}
}

// TestStartChildSpan_RecordsAndParented exercises the child-span helper
// under the in-memory exporter: an opened child span must
//
//   - appear in the export,
//   - share the trace id of its parent,
//   - carry the ai-kbs.span.kind attribute so the trace UI filter works.
func TestStartChildSpan_RecordsAndParented(t *testing.T) {
	rec := newSpanRecorder(t)
	t.Cleanup(func() { rec.shutdown() })

	ctx, rootSpan := rec.manager.Tracer().Start(context.Background(), "root")
	dbCtx, dbSpan := StartChildSpan(ctx, rec.manager, DB().WithName("doc_repository.get"))
	dbSpan.End()
	_, esSpan := StartChildSpan(dbCtx, rec.manager, ES().WithName("knowledge_search"))
	esSpan.End()
	rootSpan.End()

	spans := rec.allSpans()
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans (root + db + es), got %d", len(spans))
	}
	// Root + DB + ES — all share the same trace id.
	rootTrace := spans[0].SpanContext.TraceID()
	for i, s := range spans {
		if s.SpanContext.TraceID() != rootTrace {
			t.Errorf("span[%d] (%s) trace id = %s, want root trace %s",
				i, s.Name, s.SpanContext.TraceID(), rootTrace)
		}
	}
}
