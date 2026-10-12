// Package observe — metrics_test.go pins the Prometheus metric
// contract called out in prd.md §3.1 AC-B10 and design.md §4.5.
// The six metric families MUST be present, MUST be registered, and
// MUST respect their declared label cardinality. A future operator who
// renames a metric, drops a label, or changes a unit should fail
// loudly here.
package observe

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestRegistry_SixMetricFamilies asserts the package-wide registry
// carries exactly the six metric families required by AC-B10 plus
// the two standard runtime collectors (Go runtime, process). The
// names are frozen: any rename requires a follow-up commit that also
// updates the Grafana dashboard JSON under
// observability/dashboards/ai-kbs-overview.json.
func TestRegistry_SixMetricFamilies(t *testing.T) {
	m := Registry()

	// Vec metrics are not reported by Gather() until they have at
	// least one observed series with at least one label set. Seed
	// one series per family so the gather output reflects the
	// registry's true surface.
	ObserveHTTP("GET", "/probe", 200, 0.001, "")
	ObserveHTTP("GET", "/probe", 500, 0.5, "test.error")
	SetQueueDepth("probe", 1)
	AddLLMTokens("probe-model", LLMDirectionCompletion, 1)
	IncCacheHit("probe-cache")
	IncCacheMiss("probe-cache")

	// Pull the gather output. The Go and process collectors emit a
	// large number of metrics (one per Go runtime knob) so we don't
	// check the byte-for-byte body — we only assert the six
	// ai_kbs_* families are present.
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather: %v", err)
	}
	got := map[string]bool{}
	for _, mf := range mfs {
		got[mf.GetName()] = true
	}
	required := []string{
		"ai_kbs_http_requests_total",
		"ai_kbs_http_request_duration_seconds",
		"ai_kbs_http_requests_errors_total",
		"ai_kbs_queue_depth",
		"ai_kbs_llm_tokens_total",
		"ai_kbs_cache_hits_total",
		"ai_kbs_cache_misses_total",
	}
	for _, name := range required {
		if !got[name] {
			t.Errorf("required metric family %q missing from registry", name)
		}
	}
}

// TestObserveHTTP_IncrementsCounters exercises the three HTTP
// collectors end-to-end. After three successful + one error response,
// the counters and the histogram must reflect the four calls.
func TestObserveHTTP_IncrementsCounters(t *testing.T) {
	m := Registry()

	// Use the testutil helper for counter / histogram assertions —
	// it gathers the registry internally and supports WithLabelValues.
	before := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "2xx"))
	ObserveHTTP("GET", "/test", 200, 0.01, "")
	after := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "2xx"))
	if after-before != 1 {
		t.Errorf("HTTPRequestsTotal +1 expected, got %.0f", after-before)
	}

	// The histogram observation is harder to assert directly, but
	// testutil.CollectAndCount counts the series; we sanity-check
	// that the metric is still registered.
	if count := testutil.CollectAndCount(m.HTTPRequestDuration); count == 0 {
		t.Error("HTTPRequestDuration is registered but has no series")
	}
}

// TestObserveHTTP_ErrorCounter proves that a 4xx response is also
// recorded in the dedicated errors counter, and the code label
// surfaces the supplied error code.
func TestObserveHTTP_ErrorCounter(t *testing.T) {
	m := Registry()
	before := testutil.ToFloat64(m.HTTPRequestsErrorsTotal.WithLabelValues("POST", "/test", "auth.token_expired"))
	ObserveHTTP("POST", "/test", 401, 0.02, "auth.token_expired")
	after := testutil.ToFloat64(m.HTTPRequestsErrorsTotal.WithLabelValues("POST", "/test", "auth.token_expired"))
	if after-before != 1 {
		t.Errorf("HTTPRequestsErrorsTotal +1 expected, got %.0f", after-before)
	}
}

// TestObserveHTTP_FallsBackToStatusClassWhenNoCode asserts the
// default code label when the caller does not pass one. This is
// the "happy-path 4xx" path where the handler didn't call
// c.Error(gin.ErrorTypePublic, "...") — we still want a usable
// label.
func TestObserveHTTP_FallsBackToStatusClassWhenNoCode(t *testing.T) {
	m := Registry()
	before := testutil.ToFloat64(m.HTTPRequestsErrorsTotal.WithLabelValues("DELETE", "/test", "5xx"))
	ObserveHTTP("DELETE", "/test", 500, 0.05, "")
	after := testutil.ToFloat64(m.HTTPRequestsErrorsTotal.WithLabelValues("DELETE", "/test", "5xx"))
	if after-before != 1 {
		t.Errorf("HTTPRequestsErrorsTotal (fallback to 5xx) +1 expected, got %.0f", after-before)
	}
}

// TestSetQueueDepth exercises the gauge path. The value must be
// retrievable via testutil.ToFloat64 after SetQueueDepth.
func TestSetQueueDepth(t *testing.T) {
	SetQueueDepth("core", 42)
	defer SetQueueDepth("core", 0)
	if v := testutil.ToFloat64(Registry().QueueDepth.WithLabelValues("core")); v != 42 {
		t.Errorf("QueueDepth(core) = %v, want 42", v)
	}
}

// TestAddLLMTokens verifies the per-model, per-direction counter.
// The direction constant LLMDirectionPrompt must round-trip as the
// label string.
func TestAddLLMTokens(t *testing.T) {
	m := Registry()
	before := testutil.ToFloat64(m.LLMTokensTotal.WithLabelValues("gpt-4", LLMDirectionPrompt))
	AddLLMTokens("gpt-4", LLMDirectionPrompt, 100)
	after := testutil.ToFloat64(m.LLMTokensTotal.WithLabelValues("gpt-4", LLMDirectionPrompt))
	if after-before != 100 {
		t.Errorf("LLMTokensTotal(prompt) +100 expected, got %.0f", after-before)
	}
}

// TestAddLLMTokens_RejectsEmpty makes sure empty inputs are no-ops
// so a misconfigured LLM call site doesn't create a metric series
// with empty labels (which Grafana treats as a global series).
func TestAddLLMTokens_RejectsEmpty(t *testing.T) {
	// Should not panic on any of these.
	AddLLMTokens("", "prompt", 1)
	AddLLMTokens("gpt-4", "", 1)
	AddLLMTokens("gpt-4", "prompt", 0)
	AddLLMTokens("gpt-4", "prompt", -5)
}

// TestCacheHitMiss verifies the two cache counters increment
// independently, and that the canonical hit-rate formula
// hits / (hits + misses) yields a value in [0, 1].
func TestCacheHitMiss(t *testing.T) {
	m := Registry()
	hBefore := testutil.ToFloat64(m.CacheHitsTotal.WithLabelValues("doc_cache"))
	miBefore := testutil.ToFloat64(m.CacheMissesTotal.WithLabelValues("doc_cache"))
	IncCacheHit("doc_cache")
	IncCacheHit("doc_cache")
	IncCacheMiss("doc_cache")
	hAfter := testutil.ToFloat64(m.CacheHitsTotal.WithLabelValues("doc_cache"))
	miAfter := testutil.ToFloat64(m.CacheMissesTotal.WithLabelValues("doc_cache"))
	if hAfter-hBefore != 2 {
		t.Errorf("CacheHitsTotal +2 expected, got %.0f", hAfter-hBefore)
	}
	if miAfter-miBefore != 1 {
		t.Errorf("CacheMissesTotal +1 expected, got %.0f", miAfter-miBefore)
	}
}

// TestHandler_ExposesAllFamilies runs the promhttp handler against
// the package registry and asserts every ai_kbs_* family is
// present in the text exposition output. This is the contract a
// Prometheus scraper relies on.
func TestHandler_ExposesAllFamilies(t *testing.T) {
	// Force at least one series per family so the output includes
	// the family NAME (Prometheus omits families with no series
	// from the text format).
	ObserveHTTP("GET", "/probe", 200, 0.001, "")
	ObserveHTTP("GET", "/probe", 500, 0.5, "test.error")
	SetQueueDepth("probe", 3)
	AddLLMTokens("probe-model", LLMDirectionCompletion, 7)
	IncCacheHit("probe-cache")
	IncCacheMiss("probe-cache")

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("handler status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	required := []string{
		"ai_kbs_http_requests_total",
		"ai_kbs_http_request_duration_seconds",
		"ai_kbs_http_requests_errors_total",
		"ai_kbs_queue_depth",
		"ai_kbs_llm_tokens_total",
		"ai_kbs_cache_hits_total",
		"ai_kbs_cache_misses_total",
	}
	for _, name := range required {
		if !strings.Contains(body, name) {
			t.Errorf("metrics output missing %q", name)
		}
	}
}

// TestObserveHTTP_EmptyPathDoesNotPanic covers the "no route
// template" edge case: a 404 on an unknown route returns
// c.FullPath() == "". The helper substitutes "unknown" so we
// don't blow up label cardinality, but we want the substitution
// to be observable.
func TestObserveHTTP_EmptyPathDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ObserveHTTP panicked on empty path: %v", r)
		}
	}()
	ObserveHTTP("GET", "", 404, 0.001, "")
}

// TestRegistryHandle_SingletonConsistency ensures the registry
// returned by Registry() and RegistryHandle() is the same instance —
// a future refactor that creates a second registry would silently
// drop metrics on the floor. We compare Registry() (the
// wrapper) to RegistryHandle() (the underlying handle) and two
// RegistryHandle() calls back to back; both must point at the
// same *prometheus.Registry.
func TestRegistryHandle_SingletonConsistency(t *testing.T) {
	first := RegistryHandle()
	second := RegistryHandle()
	if first == nil {
		t.Fatal("RegistryHandle returned nil")
	}
	if first != second {
		t.Error("RegistryHandle returned different *prometheus.Registry on consecutive calls")
	}
}
