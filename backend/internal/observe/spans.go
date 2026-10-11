package observe

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// SpanKind enumerates the canonical child-span categories required by
// prd.md §3.1 AC-B8: "DB / ES / SAG / LLM 调用产生 child span". The
// labels feed the Langfuse UI filter dropdowns (and any future
// aggregator that splits the trace tree by category), so they are part
// of the public observability surface and must NOT drift without notice
// — see config_test.go::TestSpanKindLabels.
//
// Naming style: lowercase snake_case, no prefixes (the OTel resource
// already carries service.name=ai-kbs, so prefixing every child would
// just duplicate information).
type SpanKind string

const (
	// KindDB is the canonical name for child spans wrapping a Postgres
	// / pgx call (SELECT/INSERT/UPDATE/CAS update).
	KindDB SpanKind = "db"
	// KindES is the canonical name for child spans wrapping an
	// Elasticsearch / OpenSearch call (search / index / bulk).
	KindES SpanKind = "es"
	// KindSAG is the canonical name for child spans wrapping a SAG
	// (Semantic-Augmented Generation) service call (graph query /
	// retrieval enrichment).
	KindSAG SpanKind = "sag"
	// KindLLM is the canonical name for child spans wrapping an LLM
	// call (chat completion / embedding / rerank / ASR / VLM).
	KindLLM SpanKind = "llm"
	// KindHTTP is the canonical name for child spans wrapping an
	// outbound HTTP request (e.g. fetch an upstream API key, callback
	// to an IM channel).
	KindHTTP SpanKind = "http"
	// KindAsynq is the canonical name for child spans wrapping an
	// asynq enqueue / dequeue (cross-process correlation between the
	// HTTP request that enqueued a task and the worker that handles it).
	// tracing/langfuse/asynq.go already names its spans "asynq.<type>";
	// KindAsynq is the category label attached alongside it.
	KindAsynq SpanKind = "asynq"
)

// ChildSpan is the typed metadata for opening a child span around an
// infrastructure call. Constructing one does NOT open the span — pass
// it to the langfuse Manager; this struct exists so business code only
// sees the canonical "kind / name" pair (not raw string attributes that
// could drift between call sites).
//
// The configured OTLP exporter (Langfuse backend) renders the kind
// underneath the root HTTP span in the trace UI; the name flows into
// the langfuse.observation.type attribute (set to "span" — child spans
// are "span", not "generation"; only LLM calls are "generation").
type ChildSpan struct {
	// Kind is the categorical label (db / es / sag / llm / http /
	// asynq).
	Kind SpanKind
	// Name is a short human-readable identifier (e.g. "doc_repository.get",
	// "embed_index.search"). Prepend the resource this package ships on:
	// "ai-kbs.<kind>.<resource>" — see DB()/ES()/SAG()/LLM().
	Name string
	// Description is a free-form multi-line summary suitable for the
	// Langfuse "input" field. Optional; empty is valid.
	Description string
}

// DB describes a Postgres / pgx child span. The Name is fixed at
// "ai-kbs.db" so call sites that want a finer-grained label can
// override it via a ChildSpan literal instead of calling this helper.
func DB() ChildSpan {
	return ChildSpan{Kind: KindDB, Name: "ai-kbs.db"}
}

// ES describes an Elasticsearch / OpenSearch child span.
func ES() ChildSpan {
	return ChildSpan{Kind: KindES, Name: "ai-kbs.es"}
}

// SAG describes a SAG (semantic-augmented generation) child span.
func SAG() ChildSpan {
	return ChildSpan{Kind: KindSAG, Name: "ai-kbs.sag"}
}

// LLM describes an LLM child span (chat / embedding / rerank / ASR /
// VLM). Note: actual LLM call sites should use tracing/langfuse's
// Manager.StartGeneration (which emits obsType=generation, and records
// token usage) rather than this generic ChildSpan helper. LLM() is
// here for completeness so the typed helpers cover every category the
// AC-B8 acceptance criterion lists.
func LLM() ChildSpan {
	return ChildSpan{Kind: KindLLM, Name: "ai-kbs.llm"}
}

// String renders kind for logs / metrics labels. Empty kind returns
// "unknown" so an unset observability layer doesn't drop the label
// entirely when serialised.
func (k SpanKind) String() string {
	if k == "" {
		return "unknown"
	}
	return string(k)
}

// String renders name for logs / span-name fields. Falls through to a
// "<kind>.<auto>" placeholder if both fields are empty, so a stray
// ChildSpan{} doesn't produce a span with literally no name in the
// trace UI.
func (c ChildSpan) String() string {
	if c.Name != "" {
		return c.Name
	}
	if c.Kind != "" {
		return string(c.Kind) + ".auto"
	}
	return "ai-kbs.unknown"
}

// StartChildSpan opens an OTel child span under the in-flight root
// span (if any) for the supplied ChildSpan metadata and returns a
// derived ctx + the OTel span handle. When observability is disabled
// (mgr nil or Enabled()==false) the call is a cheap pass-through:
// ctx is returned unchanged and the returned *ChildSpan handle wraps a
// no-op span, so callers never need a nil check.
//
// Usage:
//
//	ctx, span := observe.StartChildSpan(ctx, observe.DB().
//	    WithName("doc_repository.get"),
//	    observe.WithAttribute("doc.code", code))
//	defer span.End()
//	result, err := repo.Get(ctx, code)
//	if err != nil { span.RecordError(err) }
//
// The returned *ChildSpan handle's End() method MUST be called via
// defer so the span is exported on every code path (success, error,
// panic-with-recover). WithName / WithDescription are convenience
// builders used inline at the call site (Go doesn't support default
// arguments; see below).
func StartChildSpan(ctx context.Context, mgr *Manager, spec ChildSpan, opts ...ChildSpanOption) (context.Context, *ChildSpanHandle) {
	if mgr == nil || !mgr.Enabled() || ctx == nil {
		return ctx, &ChildSpanHandle{}
	}
	for _, opt := range opts {
		opt(&spec)
	}
	spanName := spec.String()
	spanCtx, otelSpan := mgr.Tracer().Start(ctx, spanName,
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
		oteltrace.WithAttributes(
			attribute.String("ai-kbs.span.kind", spec.Kind.String()),
			attribute.String("ai-kbs.span.category", string(spec.Kind)),
		),
	)
	if spec.Description != "" {
		otelSpan.SetAttributes(attribute.String("ai-kbs.span.description", spec.Description))
	}
	return spanCtx, &ChildSpanHandle{
		span:   otelSpan,
		mgr:    mgr,
		wrapped: true,
	}
}

// ChildSpanHandle wraps an OTel child span for safe completion. Its
// methods are nil-safe: when the underlying span wasn't opened
// (observability disabled), every method is a no-op.
//
// End() MUST be called via defer. Optional RecordError / SetAttributes
// hooks make the call site concise:
//
//	ctx, span := observe.StartChildSpan(ctx, mgr, observe.LLM())
//	defer span.End()
//	out, err := model.Chat(ctx, msg)
//	if err != nil {
//	    span.RecordError(err)   // marks span ERROR, attaches event
//	    return out, err
//	}
//	span.SetAttributes(attribute.Int("llm.tokens", out.Total))
//	return out, nil
type ChildSpanHandle struct {
	span    oteltrace.Span
	mgr     *Manager
	wrapped bool
}

// End finishes the child span. Safe to call on a nil / no-op handle.
func (h *ChildSpanHandle) End() {
	if h == nil || h.span == nil {
		return
	}
	h.span.End()
}

// RecordError records an error event on the child span and marks its
// status as ERROR. Idempotent: multiple errors collapse into multiple
// events (the first sets the status). Safe on a no-op handle.
func (h *ChildSpanHandle) RecordError(err error) {
	if h == nil || h.span == nil || err == nil {
		return
	}
	h.span.RecordError(err)
	h.span.SetStatus(codes.Error, err.Error())
}

// SetAttributes attaches key-value pairs from a map to the child span.
// Use this for the "finish metadata" known only at completion time
// (token counts, query duration, etc).
func (h *ChildSpanHandle) SetAttributes(attrs map[string]string) {
	if h == nil || h.span == nil || len(attrs) == 0 {
		return
	}
	kv := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kv = append(kv, attribute.String(k, v))
	}
	h.span.SetAttributes(kv...)
}

// ChildSpanOption configures a ChildSpan at construction time.
// Implementations live in the same package so call sites stay terse.
type ChildSpanOption func(*ChildSpan)

// WithName overrides the ChildSpan name. Use for fine-grained labels
// like "doc_repository.get" while keeping the Kind categorical.
func WithName(name string) ChildSpanOption {
	return func(c *ChildSpan) { c.Name = name }
}

// WithDescription attaches a free-form description to the child span.
// Truncated to 1 KB so a 1 MB tool result cannot flood the trace UI.
func WithDescription(desc string) ChildSpanOption {
	const maxLen = 1024
	if len(desc) > maxLen {
		desc = desc[:maxLen] + "..."
	}
	return func(c *ChildSpan) { c.Description = desc }
}

// ChildSpan.WithName is a method-chained shorthand so the helpers DB(),
// ES(), SAG(), LLM() can be parameterised at the call site without a
// import-only-for-types detour into options.go.
func (c ChildSpan) WithName(name string) ChildSpan {
	c.Name = name
	return c
}

// ChildSpan.WithDescription mirrors WithName for the description field.
func (c ChildSpan) WithDescription(desc string) ChildSpan {
	c.Description = desc
	return c
}
