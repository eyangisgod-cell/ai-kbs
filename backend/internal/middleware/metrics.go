// Package middleware — metrics.go installs the Gin middleware that
// records QPS / latency / error rate for every request. The three
// collectors live in internal/observe/metrics.go (T1.21); this file
// is the glue that turns gin.Context into a single ObserveHTTP call.
//
// The middleware runs AFTER the request_id / language / recovery
// middlewares (so c.FullPath() resolves to the route template and the
// recovery layer has already converted a panic to a 5xx response) and
// BEFORE the auth / RBAC layers (so a 401 is also recorded; otherwise
// an attacker probing for authn gaps would be invisible).
//
// Cardinality discipline: c.FullPath() returns the gin route template
// (e.g. "/api/v1/users/:id"), not the raw URL. We use it as-is —
// anything else (raw URL, normalised URL) explodes the series count
// and is the most common production scrape failure.
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/eyangisgod-cell/ai-kbs/internal/observe"
)

// HTTPMetrics returns the gin middleware that records the three HTTP
// metric families (QPS, latency, error rate) for every request. It
// also forwards the c.Writer.Status() and the optional c.Errors first
// element as the error_code label so the error-rate panel can split
// 4xx / 5xx by category.
//
// The middleware is safe to install multiple times — the collectors
// are package-level singletons so duplicate installs only add
// redundant work, not duplicate series.
func HTTPMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Stash the request id (if any) so the error-code label can
		// fall back to the response status class when no c.Errors
		// entry is set by the handler.
		c.Next()

		// Compute duration after the handler chain returns. Using
		// time.Since on a captured start time avoids a subtle bug
		// where a clock-jump between c.Next() and the post-processing
		// block would skew the histogram.
		duration := time.Since(start).Seconds()

		method := c.Request.Method
		path := c.FullPath()
		status := c.Writer.Status()
		code := errorCodeFromContext(c, status)

		observe.ObserveHTTP(method, path, status, duration, code)
	}
}

// errorCodeFromContext picks the canonical error code for the
// http_requests_errors_total{code="..."} label. Order of preference:
//
//  1. The first c.Errors entry, if it carries a non-empty string —
//     this is the code an upstream error_handler middleware has
//     already attached (e.g. "auth.token_expired").
//  2. The HTTP status class (e.g. "5xx") so the metric still has a
//     valid label even when the handler did not register an error.
func errorCodeFromContext(c *gin.Context, status int) string {
	if c != nil {
		for _, e := range c.Errors {
			if e == nil {
				continue
			}
			// gin.Error exposes .Error (the error itself) and .Type
			// (gin.ErrorTypePublic / Private / Render). Public
			// codes are operators' first debugging surface.
			if e.Type == gin.ErrorTypePublic && e.Meta != nil {
				if code, ok := e.Meta.(string); ok && code != "" {
					return code
				}
			}
			if e.Error() != "" {
				return e.Error()
			}
		}
	}
	return statusClass(status)
}

// statusClass returns the class label ("4xx", "5xx", ...) for status
// codes. We mirror observe.intToStatusLabel by hand here so this
// file does not need to expose an internal helper; the two stay in
// lockstep by the metrics_test.go test that asserts ObserveHTTP
// produces the same class.
func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}
