package observe

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// propagationHeader is the W3C `traceparent` header key. Kept package-
// local so the constant stays in sync with the parser in traceparent.go.
const propagationHeader = "traceparent"

// otelTracerName is the name reported in span.scope.name for ai-kbs
// root spans. Lets reviewers filter by scope in the trace UI.
const otelTracerName = "github.com/eyangisgod-cell/ai-kbs/internal/observe"

// ctxKeyTraceParentRecord is the context key we store the parsed
// Traceparent on (or its zero value if absent). Used by StampTraceID
// without re-running ParseTraceparent.
type ctxKeyTraceParentRecord struct{}

// rootSpanCtxKey is the context key for the OTel root span. Downstream
// helpers (envelope stamper, root-span attributer) can pull it out
// without re-running propagator extract.
type rootSpanCtxKey struct{}

// W3C TraceContext propagator instance. We do NOT mutate the OTel
// global propagator (otel.SetTextMapPropagator) from this package —
// that would collide with any other OTel instrumentation in the
// same process. We hand this propagator instance directly to Extract.
var w3cPropagator = propagation.TraceContext{}

// requestIDContextKey mirrors the request_id key installed by
// middleware.RequestID. We re-declare the key as a private type so this
// package doesn't import internal/types directly (which would create a
// types → observe → types cycle through middleware ordering).
type requestIDContextKey struct{}

// requestIDFromContext pulls a request id stamped by the upstream
// middleware. Returns ("", false) when no request id is on ctx — the
// middlewares in this package tolerate that case (the request_id
// attribute is best-effort, not load-bearing).
func requestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(requestIDContextKey{}).(string)
	return v, ok && v != ""
}

// TraceIDFromContext returns the W3C trace id parsed from the incoming
// request's `traceparent` header (after RootSpanMiddleware has run).
// Returns ("", false) when no upstream traceparent was supplied, when
// the header was malformed, or when RootSpanMiddleware did not run on
// this handler chain.
//
// Callers — typically the envelope builder at the very end of the
// handler chain — use it to stamp the envelope's optional trace_id
// field per design.md §3.1 ("trace_id": "<w3c-trace-id-32hex>").
func TraceIDFromContext(c *gin.Context) (string, bool) {
	if c == nil || c.Request == nil {
		return "", false
	}
	return traceIDFromCtx(c.Request.Context())
}

func traceIDFromCtx(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	tp, ok := ctx.Value(ctxKeyTraceParentRecord{}).(Traceparent)
	if !ok || tp.TraceID == "" {
		return "", false
	}
	return tp.TraceID, true
}

// RootSpanFromContext returns the OTel root span opened by
// RootSpanMiddleware, or nil if the middleware did not run (or
// observability is disabled). Callers use it to add dynamic attributes
// (request_id, db.query_id, ...) to the in-flight root span without
// having to thread the *oteltrace.Span through the request context by
// hand.
//
// The returned span MUST NOT be Ended by the caller — the middleware
// calls End after the handler chain returns so the recorded status /
// attributes / events arrive in the trace UI once, not twice.
func RootSpanFromContext(ctx context.Context) oteltrace.Span {
	if ctx == nil {
		return nil
	}
	sp, ok := ctx.Value(rootSpanCtxKey{}).(oteltrace.Span)
	if !ok {
		return nil
	}
	return sp
}

// RootSpanMiddleware returns a Gin handler that opens a root span for
// EVERY incoming HTTP request.
//
// Per prd.md §3.1 AC-B8 ("any HTTP request in the Langfuse dashboard
// shows the complete span tree"), the filter is intentionally permissive —
// health checks, swagger, static assets — all surface in the trace UI at
// at least the root-span level so an operator can answer "did the
// request reach the service?". Filtering by kind is left to the trace
// UI / Prometheus (T1.21), not to this middleware; the design.md §3.1
// contract is one-tree-per-request.
//
// When the OTel observability manager is disabled (env not configured)
// or nil, the middleware degrades to a W3C parse-and-record pass-through
// so handlers downstream can still stamp the envelope trace_id via
// TraceIDFromContext.
//
// Inheritance: when the incoming request carries a `traceparent`
// header, the extracted SpanContext is set on ctx BEFORE we open the
// root span — so this span automatically becomes a child of the
// upstream caller's span (parent link is automatic through OTel
// propagation; middleware does not mutate the upstream trace id).
func RootSpanMiddleware(mgr *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Step 1: parse the upstream traceparent header BEFORE any
		// span opens, so handlers can stamp the envelope even when
		// observability is disabled.
		tp := ParseTraceparent(c.GetHeader(propagationHeader))
		if tp.TraceID != "" {
			ctx = context.WithValue(ctx, ctxKeyTraceParentRecord{}, tp)
			c.Request = c.Request.WithContext(ctx)
		}

		// Step 2: when observability is disabled / nil, skip span
		// emission. The parsed trace id is already on ctx (above) so
		// envelope helpers downstream still work.
		if mgr == nil || !mgr.Enabled() {
			c.Next()
			return
		}

		// Step 3: extract the upstream SpanContext via the W3C
		// propagator. Doing this BEFORE tracer.Start is the
		// OTel-recommended order so the new span gets parented
		// automatically by OTel context machinery.
		parentCtx := w3cPropagator.Extract(ctx,
			propagation.HeaderCarrier(c.Request.Header))

		spanName := c.Request.Method
		if route := c.FullPath(); route != "" {
			spanName = c.Request.Method + " " + route
		}

		spanCtx, span := mgr.Tracer().Start(parentCtx, spanName,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Request.Method),
				semconv.HTTPRouteKey.String(c.FullPath()),
				semconv.URLPath(c.Request.URL.Path),
				semconv.ClientAddress(c.ClientIP()),
			),
		)
		span.SetAttributes(
			attribute.String("ai-kbs.span.kind", KindHTTP.String()),
		)
		// Record the request_id onto the span so the trace UI groups
		// spans by correlation id the same way log lines do.
		if rid, ok := requestIDFromContext(ctx); ok {
			span.SetAttributes(attribute.String("ai-kbs.span.request_id", rid))
		}

		// Stash the OTel span on ctx so any helper / handler that
		// wants to call span.RecordError can do so without re-extracting.
		ctx = context.WithValue(spanCtx, rootSpanCtxKey{}, span)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// Step 4: stamp the final attributes + finish the span after
		// the handler chain returns. Status code / response size come
		// from the gin ResponseWriter.
		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= 500 {
			span.SetStatus(codes.Error, "server error")
		}
		if size := c.Writer.Size(); size > 0 {
			span.SetAttributes(attribute.Int("http.response.body.size", size))
		}
		if errs := c.Errors; len(errs) > 0 {
			for _, e := range errs {
				span.RecordError(e.Err)
			}
			if status >= 500 {
				span.SetStatus(codes.Error, "handler returned error")
			}
		}
		span.End()
	}
}
