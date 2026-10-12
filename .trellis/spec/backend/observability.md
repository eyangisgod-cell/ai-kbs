# Observability (T1.8 — Langfuse OTLP Integration)

> Owner: backend · Status: implemented (T1.8 commit pending) ·
> Complements: `.trellis/spec/backend/error-handling.md` (envelope v1.0)

This file is the canonical spec for **how AI-KBS produces structured
traces** and ships them to a Langfuse-compatible OTLP backend. It is the
single source of truth for the env-var contract, the wire format, and
the integration patterns adopted by repository / handler / HTTP code.

---

## Why

`prd.md` §3.1 AC-B8 requires **every** HTTP request to produce a
complete span tree in the Langfuse dashboard — so an operator can
answer "did the request reach the service?", "which child spans ran?",
"what was the latency for each leg?", without re-instrumenting the
service. AC-B10 (`curl /metrics` shows six Prometheus metric classes)
is the aggregate-metrics counterpart and lives on a separate channel
(`metrics.md`, T1.21) — no overlap with this file.

The design.md §4.5 decision ("业务 span → OTLP → Langfuse; 基础设施
指标 → Prometheus → Grafana; 双通道不重叠") is enforced by:

1. **Trace ID only** travels across OTLP — Prometheus doesn't see it.
2. **Span hierarchy only** travels across OTLP — Prometheus aggregates
   by route/status, never by trace.
3. **This spec owns the OTLP channel**; `metrics.md` owns Prometheus.

---

## Architecture at a glance

```
HTTP request → middleware/observability.go
                ↓
  ParseTraceparent("traceparent" header)
                ↓
  otel SDK: Extract → Start("GET /api/v1/x", SpanKind=Server)
                ↓
  handler chain (Chat / KB / Agent / IM / ...)
                ↓
  repository call site: StartChildSpan(ctx, mgr, DB().WithName(...))
                ↓
  ES search:             StartChildSpan(ctx, mgr, ES().WithName(...))
                ↓
  SAG:                   StartChildSpan(ctx, mgr, SAG().WithName(...))
                ↓
  LLM call:              tracing/langfuse Manager.StartGeneration →

  middleware close:
   - Set HTTPResponseStatusCode
   - Span.SetStatus(codes.Error, ...) for 5xx
   - Span.End()
                ↓
  OTLP/HTTP → Langfuse v3+  (or any OTLP-compatible: Tempo / Honeycomb / OTel Collector)
```

---

## Public surface (T1.8 contract)

### Config (env vars)

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `OTLP_ENABLED` | optional | (auto) | "1"/"true"/"yes"/"on" force-enables |
| `OTLP_ENDPOINT` | when enabled | (none) | OTLP/HTTP base URL; e.g. `https://cloud.langfuse.com` |
| `OTLP_LANGFUSE_PUBLIC_KEY` | when enabled | (none) | Project public key, sent as HTTP Basic |
| `OTLP_LANGFUSE_SECRET_KEY` | when enabled | (none) | Project secret key |
| `OTLP_RELEASE` | optional | (none) | Trace UI filter (e.g. `v0.4.2`) |
| `OTLP_ENVIRONMENT` | optional | (none) | Trace UI filter (e.g. `production`) |
| `OTLP_FLUSH_AT` | optional | `15` | Span batch size |
| `OTLP_FLUSH_INTERVAL` | optional | `3s` | Accepts Go duration or integer seconds |
| `OTLP_QUEUE_SIZE` | optional | `2048` | In-memory queue bound |
| `OTLP_REQUEST_TIMEOUT` | optional | `10s` | Per-batch HTTP timeout |
| `OTLP_SAMPLE_RATE` | optional | `1.0` | Trace sampling, [0, 1] |

`OTLP_ENABLED` is implicit when **all** of `OTLP_ENDPOINT`,
`OTLP_LANGFUSE_PUBLIC_KEY`, `OTLP_LANGFUSE_SECRET_KEY` are present —
matching the Langfuse SDK convention.

### Public API (Go)

| Symbol | Where | Purpose |
|--------|-------|---------|
| `observe.Config` | `internal/observe/config.go` | Mirror of `langfuse.Config` keyed on `OTLP_*` env vars. |
| `observe.LoadConfig()` | same | Returns the resolved Config (no I/O). |
| `observe.Init(cfg) (*Manager, error)` | `internal/observe/manager.go` | Installs the singleton; builds the OTel SDK under the hood. |
| `observe.GetManager()` | same | Returns the installed singleton, or `nil`. |
| `observe.RootSpanMiddleware(mgr)` | `internal/observe/middleware.go` | **Gin middleware**: any HTTP request → root span. |
| `observe.ParseTraceparent(h)` | `internal/observe/traceparent.go` | Strict W3C parser returning the trace id (or `""` on malformed). |
| `observe.TraceIDFromContext(c)` | same | Gin-context accessor for the parsed trace id. |
| `observe.StartChildSpan(ctx, mgr, spec)` | `internal/observe/spans.go` | Opens a child span under `ctx`'s active span; returns `(ctx, *ChildSpanHandle)`. |
| `observe.DB() / ES() / SAG() / LLM() / HTTP() / Asynq()` | same | Typed factory helpers for the canonical SpanKind. |
| `observe.NewEnvelopeWithTraceID(rid, header)` | `internal/observe/envelope.go` | Builds an envelope v1.0 with the parsed trace id stamped. |

### Wire format

**`traceparent` header** (W3C TraceContext, current version):

```
00-<trace-id-32lowercasehex>-<parent-id-16lowercasehex>-<flags-2lowercasehex>
```

The parser is **strict**:

- version MUST be `"00"` (future versions ignored — see
  [W3C TraceContext §3.2.2.1](https://www.w3.org/TR/trace-context/#versioning-of-traceparent));
- trace_id MUST be 32 lowercase hex chars, NOT all-zero;
- span_id MUST be 16 lowercase hex chars, NOT all-zero;
- flags MUST be 2 lowercase hex chars.

Malformed input returns a zero `Traceparent{}` (no `TraceID`) so the
middleware mints a fresh trace — never silently propagating a bogus
id.

**Envelope v1.0** (`internal/errors/envelope/envelope.go`): the
optional `trace_id` field carries the parsed W3C trace id (omitempty —
missing entirely when none is available).

---

## Integration patterns (enforced by code review)

### Pattern 1 — Wrap any HTTP handler

```go
r := gin.New()
r.Use(observe.RootSpanMiddleware(observe.GetManager())) // AC-B8
r.GET("/api/v1/health", healthHandler)
```

Every request produces a server-kind root span named `<METHOD> <route>`,
parented under the upstream caller's span when `traceparent` is present.

### Pattern 2 — Stamp the envelope trace id

```go
env := observe.NewEnvelopeWithTraceID(rid, c.GetHeader("traceparent"))
envelope.WriteJSON(c, http.StatusOK, env)
```

Use this in every handler that builds an `*envelope.Envelope`
response. The trace id is the join key linking the HTTP response to
the trace UI.

### Pattern 3 — Wrap a DB / ES / SAG / LLM call

```go
ctx, span := observe.StartChildSpan(ctx, mgr,
    observe.DB().WithName("doc_repository.get"))
defer span.End()
out, err := repo.GetByCode(ctx, code)
if err != nil {
    span.RecordError(err)
    return err
}
span.SetAttributes(map[string]string{"doc.code": code})
```

The four canonical SpanKinds are: `db`, `es`, `sag`, `llm`. Plus
`http` for outbound HTTP and `asynq` for cross-process correlation.

### Anti-patterns (lint will catch)

- Calling `c.JSON(200, ...)` directly without going through `envelope.WriteJSON`
  bypasses the trace_id stamping. Forbidden.
- Opening a span with bare string kind (`"db"` instead of `observe.DB()`)
  risks label drift. Use the typed factories.
- Calling `span.End()` more than once per span: every End emits a new
  span. Use defer.

---

## Lifecycle of a request

1. Inbound `traceparent: 00-...` header? Extract via the W3C propagator.
2. Open a server-kind root span named `<METHOD> <route>`.
3. Stash the OTel span and the parsed `Traceparent` on `ctx`.
4. Run the handler chain (auth, RBAC, business logic, repository calls).
5. After the chain returns:
   - Set `http.response.status_code` from `c.Writer.Status()`.
   - Mark span `ERROR` for HTTP 5xx.
   - `span.End()` flushes the OTel span to the configured exporter.

The Bootstrap layer MUST call `observe.Init(observe.LoadConfig())`
during process startup so `GetManager()` returns a non-nil value
before the first `RootSpanMiddleware` call. When env vars are absent,
`Init` returns a disabled no-op manager — no startup error, no panic.

---

## Test coverage expectations

| Surface | Test file | What it covers |
|---------|-----------|----------------|
| traceparent parser | `internal/observe/traceparent_test.go` | 12 malformed-input cases + valid variants |
| envelope stamper | same | Trace id propagated to envelope, omitted when no header |
| middleware | same | Upstream trace id inherited; fresh trace id minted when absent; disabled-manager pass-through |
| child spans | same | Trace tree built; all children share root trace id |
| config loader | `internal/observe/config_test.go` | Auto-enable, explicit disable, partial-keys fail-closed, env-var parsing |
| span kind labels | same | `KindDB / KindES / KindSAG / KindLLM / KindHTTP / KindAsynq` pinned |

Run all observability tests:

```
cd backend && go test -count=1 ./internal/observe/... ./internal/tracing/...
```

---

## Cross-references

- `prd.md` §3.1 AC-B8 — the "any HTTP request produces a span tree"
  acceptance criterion this spec satisfies.
- `design.md` §3.1 — the envelope v1.0 schema (trace_id field rules).
- `design.md` §4.5 — the "双通道不重叠" decision (OTLP + Prometheus).
- `internal/tracing/langfuse/` — the OTel SDK + OTLP exporter embedded
  by `observe.Init`. Owned by the WeKnora fork; observe is the new
  AI-KBS-facing facade added in T1.8.
- `internal/errors/envelope/envelope.go` — the frozen v1.0 Envelope
  struct (`TraceID string \`json:"trace_id,omitempty"\``).
