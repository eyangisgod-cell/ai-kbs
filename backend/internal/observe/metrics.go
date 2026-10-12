// Package observe — metrics.go owns the Prometheus aggregate-metrics
// channel called out by design.md §4.5 ("双通道不重叠") and prd.md
// §3.1 AC-B10. The OTLP channel is in middleware.go (T1.8); this file
// is the parallel non-overlapping channel: it carries per-route,
// per-cache, per-queue, per-model counters and histograms that the
// Grafana dashboard aggregates into QPS / latency / error rate / queue
// depth / LLM token / cache hit rate panels.
//
// The contract is deliberately small:
//
//   - Six metric categories, named snake_case with units:
//       1. http_requests_total{method,path,status}            counter   (QPS)
//       2. http_request_duration_seconds{method,path}         histogram (latency)
//       3. http_requests_errors_total{method,path,code}      counter   (error rate)
//       4. queue_depth{queue_name}                            gauge     (queue depth)
//       5. llm_tokens_total{model,direction}                  counter   (LLM token)
//       6. cache_hits_total{cache_name} / cache_misses_total  counter   (cache hit rate)
//
//   - One package-wide Registry (Registry()) shared by every helper so
//     middleware / handler / repository sites register against the same
//     exporter.
//
//   - One Handler() that returns the canonical promhttp handler so the
//     Gin /metrics endpoint is a one-liner.
//
//   - Init must be safe to call from bootstrap without OTLP being
//     enabled — metrics are independent of tracing and MUST be usable
//     even when OTLP env vars are unset.
package observe

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metric label keys. Centralised so any call site uses the SAME string;
// label-set drift is the #1 cause of confusing Grafana queries ("wait,
// is it `route` or `path` here?"). Keep this list frozen unless you
// also update the dashboard JSON.
const (
	labelMethod    = "method"
	labelPath      = "path"
	labelStatus    = "status"
	labelCode      = "code"
	labelQueueName = "queue_name"
	labelModel     = "model"
	labelDirection = "direction"
	labelCacheName = "cache_name"
)

// HTTP latency histogram bucket boundaries (seconds). The default
// Prometheus buckets stop at 10s which hides tail latency for slow LLM
// paths. We extend up to 60s and add a fine-grained 5ms..50ms tail for
// the cache hit path.
var httpDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
}

// Metrics groups every Prometheus collector managed by the observe
// package. The zero value is intentionally NOT useful: callers must
// go through Registry() so the singleton stays canonical.
type Metrics struct {
	// HTTPRequestsTotal: QPS numerator.
	// Per design.md §4.5: aggregated by (method, path, status); the
	// `path` label uses the gin route TEMPLATE (e.g. "/api/v1/users/:id")
	// so distinct ids don't blow up cardinality.
	HTTPRequestsTotal *prometheus.CounterVec
	// HTTPRequestDuration: latency in seconds, histogram for percentile
	// estimation in Grafana. Default HistogramVec.
	HTTPRequestDuration *prometheus.HistogramVec
	// HTTPRequestsErrorsTotal: subset of HTTPRequestsTotal where
	// status >= 400; gives the "error rate" panel a cleaner denominator
	// (the panel divides by HTTPRequestsTotal, but a separate counter
	// makes the query simpler and avoids re-filtering on every scrape).
	HTTPRequestsErrorsTotal *prometheus.CounterVec
	// QueueDepth: gauge per logical asynq queue. Callers (worker
	// bootstrap or scrape-side helper) refresh the value via SetQueueDepth.
	QueueDepth *prometheus.GaugeVec
	// LLMTokensTotal: prompt vs completion tokens per model. Direction
	// is one of "prompt" / "completion" / "embedding" — see the
	// AddLLMTokens helper for the canonical strings.
	LLMTokensTotal *prometheus.CounterVec
	// CacheHitsTotal / CacheMissesTotal: hit rate numerator and
	// denominator. The dashboard panel computes
	//   rate(cache_hits_total[5m]) / rate(cache_hits_total[5m] + cache_misses_total[5m])
	CacheHitsTotal   *prometheus.CounterVec
	CacheMissesTotal *prometheus.CounterVec
	// registry is the package-private *prometheus.Registry that
	// Handler() reads from. Exposed (via Registry() in tests) so
	// callers can run their own gather/scrape against the same
	// collector set without rebuilding it.
	registry *prometheus.Registry
}

var (
	metricsOnce sync.Once
	metricsVal  *Metrics
)

// Registry returns the package-wide Prometheus metrics singleton. The
// first call registers the six metric families plus the standard Go /
// process collectors; subsequent calls return the cached value.
//
// This function is concurrency-safe and idempotent.
func Registry() *Metrics {
	metricsOnce.Do(func() {
		reg := prometheus.NewRegistry()

		// Standard Go runtime + process collectors so Grafana has
		// goroutine count, GC pause, RSS, FDs, etc. without a second
		// scrape target. These are cheap and standard.
		reg.MustRegister(collectors.NewGoCollector())
		reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

		m := &Metrics{
			HTTPRequestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "ai_kbs",
					Subsystem: "http",
					Name:      "requests_total",
					Help:      "Total number of HTTP requests served, partitioned by method, path template, and response status code.",
				},
				[]string{labelMethod, labelPath, labelStatus},
			),
			HTTPRequestDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "ai_kbs",
					Subsystem: "http",
					Name:      "request_duration_seconds",
					Help:      "HTTP request latency in seconds, partitioned by method and path template.",
					Buckets:   httpDurationBuckets,
				},
				[]string{labelMethod, labelPath},
			),
			HTTPRequestsErrorsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "ai_kbs",
					Subsystem: "http",
					Name:      "requests_errors_total",
					Help:      "Total number of HTTP responses with status >= 400, partitioned by method, path template, and error code.",
				},
				[]string{labelMethod, labelPath, labelCode},
			),
			QueueDepth: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "ai_kbs",
					Subsystem: "queue",
					Name:      "depth",
					Help:      "Number of pending tasks in the named asynq / MQ queue (sampled at scrape time).",
				},
				[]string{labelQueueName},
			),
			LLMTokensTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "ai_kbs",
					Subsystem: "llm",
					Name:      "tokens_total",
					Help:      "Cumulative LLM token usage, partitioned by model and direction (prompt / completion / embedding).",
				},
				[]string{labelModel, labelDirection},
			),
			CacheHitsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "ai_kbs",
					Subsystem: "cache",
					Name:      "hits_total",
					Help:      "Cumulative cache hits, partitioned by cache name.",
				},
				[]string{labelCacheName},
			),
			CacheMissesTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "ai_kbs",
					Subsystem: "cache",
					Name:      "misses_total",
					Help:      "Cumulative cache misses, partitioned by cache name. Hit rate is hits / (hits + misses).",
				},
				[]string{labelCacheName},
			),
			registry: reg,
		}
		reg.MustRegister(
			m.HTTPRequestsTotal,
			m.HTTPRequestDuration,
			m.HTTPRequestsErrorsTotal,
			m.QueueDepth,
			m.LLMTokensTotal,
			m.CacheHitsTotal,
			m.CacheMissesTotal,
		)
		metricsVal = m
	})
	return metricsVal
}

// RegistryHandle returns the underlying *prometheus.Registry. Used by
// the /metrics handler and by tests that need to gather metric values
// directly. Returns the same instance Registry() lazily created.
func RegistryHandle() *prometheus.Registry {
	return Registry().registry
}

// Handler returns the canonical promhttp handler that exposes the
// package registry in the text exposition format Prometheus scrapes.
// Wrap this in a Gin route — see internal/handler/metrics.go.
func Handler() http.Handler {
	return promhttp.HandlerFor(RegistryHandle(), promhttp.HandlerOpts{
		EnableOpenMetrics: false,
		Registry:          RegistryHandle(),
	})
}

// ResetForTest clears the singleton and reinitialises the registry.
// Tests that want a clean metric baseline between cases call this via
// t.Cleanup; production code MUST NOT touch it.
func ResetForTest() {
	metricsOnce = sync.Once{}
	metricsVal = nil
}

// ObserveHTTP records a single HTTP request outcome. The path argument
// MUST be the gin route template (c.FullPath()), not the raw URL —
// otherwise the label set explodes into one series per id. statusCode
// is the int written by the handler; an errorCode (e.g. "auth.token_expired")
// is passed for 4xx/5xx so the error-rate panel can split by code.
//
// Safe to call from any goroutine; the underlying collectors are
// thread-safe.
func ObserveHTTP(method, path string, statusCode int, durationSeconds float64, errorCode string) {
	m := Registry()
	if method == "" {
		method = "UNKNOWN"
	}
	if path == "" {
		path = "unknown"
	}
	statusLabel := intToStatusLabel(statusCode)
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusLabel).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(durationSeconds)
	if statusCode >= 400 {
		code := errorCode
		if code == "" {
			code = statusLabel
		}
		m.HTTPRequestsErrorsTotal.WithLabelValues(method, path, code).Inc()
	}
}

// SetQueueDepth publishes the current depth of a named queue. The
// queue_name label is a free-form string supplied by the caller
// (e.g. "core", "post_process", "enrichment", "wiki"). Pass a
// negative value to indicate "queue is offline" — Grafana renders
// it as N/A but the series stays queryable.
func SetQueueDepth(queueName string, depth int64) {
	if queueName == "" {
		return
	}
	Registry().QueueDepth.WithLabelValues(queueName).Set(float64(depth))
}

// AddLLMTokens accumulates token usage. direction MUST be one of the
// canonical strings (see below); the constant block makes accidental
// drift visible at the call site.
func AddLLMTokens(model, direction string, tokens int64) {
	if model == "" || direction == "" || tokens <= 0 {
		return
	}
	Registry().LLMTokensTotal.WithLabelValues(model, direction).Add(float64(tokens))
}

// Direction labels for AddLLMTokens. Kept as exported constants so
// LLM-call sites pass a typed value (not a magic string).
const (
	LLMDirectionPrompt     = "prompt"
	LLMDirectionCompletion = "completion"
	LLMDirectionEmbedding  = "embedding"
)

// IncCacheHit / IncCacheMiss are the cache-hit-rate numerators.
// Pair them: every IncCacheHit call MUST be matched by exactly one
// IncCacheMiss-or-equal-hit at the same call site (the cache lookup
// function). Anything else skews the hit rate.
func IncCacheHit(cacheName string) {
	if cacheName == "" {
		return
	}
	Registry().CacheHitsTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheMiss(cacheName string) {
	if cacheName == "" {
		return
	}
	Registry().CacheMissesTotal.WithLabelValues(cacheName).Inc()
}

// intToStatusLabel renders the int status code as a short string the
// label set is grouped by. We deliberately use the class ("2xx", "4xx",
// "5xx") rather than the literal code so cardinality stays bounded;
// the error_code label carries the fine-grained value when present.
func intToStatusLabel(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "other"
	}
}
