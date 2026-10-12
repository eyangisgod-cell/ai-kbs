// Package handler — metrics.go exposes the Prometheus scrape endpoint
// (GET /metrics) per prd.md §3.1 AC-B10 and design.md §4.5. The
// handler is intentionally a thin wrapper around observe.Handler():
// the canonical registry is owned by the observe package, the handler
// is responsible only for adapting the request/response cycle to Gin.
//
// /metrics is registered WITHOUT auth by design (see router.go):
// Prometheus scrapers cannot carry our HMAC / JWT / APIKey
// credentials, and putting a token in the scrape config would be
// both fragile and a security risk. Network-level controls
// (firewall / service-mesh policy) keep the endpoint reachable only
// from the monitoring subnet.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/eyangisgod-cell/ai-kbs/internal/observe"
)

// MetricsHandler is the HTTP handler that exposes the observe package
// Prometheus registry in the text exposition format. The struct is
// stateless; we keep it as a type so the routing layer can mount it
// via the existing handler-package convention (e.g. handler.Metrics{}).
type MetricsHandler struct{}

// NewMetricsHandler returns a handler ready to be mounted on Gin.
// A constructor is used (not just a function literal) so the
// routing layer can also dependency-inject future options (auth
// bypass, named registry, etc.) without changing call sites.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// Metrics is the gin handler bound to GET /metrics. It delegates
// the response body to observe.Handler() — the same one Prometheus
// client_golang's promhttp.Handler() would emit. The Content-Type
// is set by the promhttp handler itself (text/plain; version=0.0.4).
func (h *MetricsHandler) Metrics(c *gin.Context) {
	observe.Handler().ServeHTTP(c.Writer, c.Request)
}

// Ping is a tiny companion endpoint useful for synthetic monitoring.
// It returns 200 OK and a one-line body so a Prometheus blackbox
// exporter or a curl-based liveness probe can confirm the scrape
// surface is reachable without parsing the entire metrics payload.
func (h *MetricsHandler) Ping(c *gin.Context) {
	c.String(http.StatusOK, "ai-kbs metrics endpoint\n")
}
