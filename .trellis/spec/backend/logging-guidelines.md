# Logging Guidelines

> How the AI-KBS backend emits logs and traces.

---

## Overview

AI-KBS uses a **dual-channel observability stack**:

1. **Structured application logs** → stdout (JSON, 12-factor) → log shipper
2. **Distributed traces + spans** → OTLP/HTTP → Langfuse

The two channels share a correlation key (`trace_id`, W3C) so a trace in
Langfuse can pivot to its application logs, and vice versa. They do NOT
overlap: traces carry span attributes, logs carry event records.

| Channel | Library | Sink | What goes here |
|---------|---------|------|----------------|
| Logs    | `go.uber.org/zap` (preferred) or `github.com/sirupsen/logrus` (legacy) | stdout → container log driver | Startup, request lifecycle events, business events, errors |
| Traces  | `go.opentelemetry.io/otel` + OTLP/HTTP exporter | Langfuse (`LANGFUSE_OTLP_ENDPOINT`) | Per-request span tree, DB query spans, LLM call spans |
| Metrics | Prometheus client | `/metrics` endpoint | Aggregate counters/histograms (NOT per-event) |

See `.trellis/tasks/10-09-M1-backend-cli/design.md` §4.5 for the dual-channel
rationale — traces for individual request drill-down, metrics for
fleet-wide aggregate, logs as the connective tissue.

---

## Structured Logging

### Format

- JSON to stdout, one event per line.
- Required fields on every log line:

```json
{
  "ts":       "2026-10-09T12:34:56.789Z",
  "level":    "info",
  "msg":      "document updated",
  "trace_id": "01JABCDEF...",        // W3C trace id (32 hex), OTLP path only
  "request_id": "01JABCDEF...",       // ULID generated per request, always present
  "user_id":  "u_abc123",
  "tenant_id": "t_xyz789",
  "doc_code": "aB3xY7zQ",
  "duration_ms": 47
}
```

`trace_id` is the W3C trace id and only present when the request is on the
OTLP path (i.e., a span was opened by middleware). `request_id` is always
present and is a ULID minted at the HTTP middleware boundary.

### Initialization

- One process-wide `*zap.Logger` from `internal/observe.NewLogger(cfg)`.
- Call sites get a child logger via `logger.With("component", "auth.hmac")`.
- Never construct `zap.NewExample()` / `zap.NewDevelopment()` ad-hoc inside a
  handler — they bypass config (encoding, level, sampling).

### Logger naming

`component` field uses `package.symbol` notation:

| Component | Where |
|-----------|-------|
| `auth.hmac` | `internal/auth/hmac.go` |
| `auth.jwt` | `internal/auth/jwt.go` |
| `auth.apikey` | `internal/auth/apikey.go` |
| `rbac.guard` | `internal/rbac/middleware.go` |
| `doc.cas` | `internal/doc/cas.go` |
| `repository.document` | `internal/repository/document.go` |
| `cli.pull` | `internal/cli/pull.go` |
| `sync.push` | `internal/sync/push.go` |
| `observe.otlp` | `internal/observe/otlp.go` |
| `observe.metrics` | `internal/observe/metrics.go` |

---

## Log Levels

| Level | When to use | Examples |
|-------|-------------|----------|
| `debug` | Verbose state useful for diagnosing a specific bug; off in prod by default | cache hit/miss detail, query plan, retry attempts |
| `info`  | Normal lifecycle events the operator wants to see | server started, request completed, document updated, user logged in |
| `warn`  | Recoverable abnormal condition that the operator should know about | retry succeeded, deprecated API used, rate-limit approaching |
| `error` | Operation failed; needs investigation but service is still up | DB query error, upstream call failed, CAS conflict (after retry) |

### Rules

1. **Errors that affect user-visible behavior are `error`**, not `warn`. A
   failed write that returns 5xx is `error`. A successful retry that the
   user didn't notice is `warn`.
2. **Startup misconfiguration is `error`** (e.g., missing `AIKBS_DB_URL`).
   The server still exits, but the log line is what the operator greps for.
3. **Per-request span errors are recorded on the span**, not as separate
   `error` log lines. Use `span.RecordError(err)` then `span.SetStatus(codes.Error, ...)`.
   A matching `error` log line at handler exit IS appropriate — log + trace
   stay correlated by `trace_id`.
4. **Do not log at `info` on every request** when the OTLP path already
   captures the request as a span. The trace is the source of truth; the
   log line is for grep-ability of specific events (e.g., "document updated"
   rather than "request received").

---

## Required Fields Per Component

The middleware populates `trace_id` (when on OTLP path) and `request_id`
on every line emitted inside the request scope. Components add their own
domain fields on top:

| Component | Extra required fields |
|-----------|-----------------------|
| `auth.*` | `user_id`, `tenant_id`, `auth_method` (`hmac` \| `jwt` \| `apikey`) |
| `rbac.*` | `user_id`, `role`, `resource`, `action`, `allowed` |
| `doc.cas` | `doc_code`, `base_version`, `new_version`, `result` (`ok` \| `conflict`) |
| `repository.*` | `query` (parameterized — see section 7), `duration_ms`, `row_count` |
| `sync.*` | `binding`, `file_count`, `direction` (`pull` \| `push`), `conflict_count` |
| `observe.*` | `exporter` (`otlp` \| `prom`), `endpoint`, `error` |

`request_id` is the join key for cross-component log queries
(`loki | json | request_id="..."` or equivalent).

---

## What MUST NOT Be Logged

These are scrubbed at the logger middleware boundary, not at the call
site — see `backend/internal/middleware/logger.go` for the
`SanitizeBody` implementation. The rules below are the **policy** the
implementation enforces; never rely on call sites to mask.

### Hard forbid (any context)

| Field | Why |
|-------|-----|
| `key_secret` (CLI credential) | Bearer equivalent; disclosure = full impersonation |
| `password` | Plain-text credential |
| `apiKey`, `api_key`, `APIKey` | LLM provider key; financial exposure |
| `secretKey`, `secret_key` | Provider secret |
| `accessToken`, `refreshToken`, `idToken` | OAuth tokens |
| `authorization` (raw header) | Contains all of the above |
| `cookie` (raw header) | Session hijack |
| `privateKey`, `private_key`, `BEGIN PRIVATE KEY` | Cryptographic material |
| `*.jwt` (full token) | Bearer equivalent |
| PAN, CVV, full bank account | PCI scope |

### Soft forbid (PII; mask unless contract requires it)

- Email addresses (mask local-part: `j***@example.com`)
- Phone numbers (mask middle digits)
- IP addresses (mask last octet for IPv4, last 80 bits for IPv6)
- Names in `error.message` (use `user_id` instead)

### Operational rule

When in doubt, log the **identifier** (`u_abc123`) and not the **value**
(`"alice@corp.com"`). Identifiers are joinable across logs; values are
not, and they leak.

---

## Trace Propagation (W3C)

Every HTTP request opens a span via `internal/middleware.Tracing` (T1.8 in
the M1 plan). The middleware:

1. Reads `traceparent` header if present (W3C `00-<trace_id>-<span_id>-<flags>`).
   Otherwise mints a new trace id (ULID-prefixed hex).
2. Creates a child span named `http.<METHOD> <route>` (e.g.,
   `http.GET /api/ai-kbs/documents/:docCode`).
3. Stores the trace id on the Gin context (`c.Set("trace_id", ...)`).
4. Sets the `traceparent` response header so the client can correlate.

Async work (asynq tasks in the existing fork) preserves trace context via
`internal/types.TracingContext` — the `LangfuseTraceparent` field carries
the W3C `traceparent` so a worker span is a child of the request that
enqueued it. New code MUST embed `TracingContext` in any asynq payload
struct that crosses the request boundary.

---

## Log Sampling

The OTLP path samples per-trace (always-on in dev, head-based 10% in
prod, configurable via `OTEL_TRACES_SAMPLER_ARG`). Application logs are
NOT sampled — operators need them for incident response.

If a hot path produces too many `info` lines (e.g., a per-request
"request completed" log on a busy endpoint), the fix is to drop the log
line in favor of the metric, not to add sampling to the logger.

---

## What Goes Where (Decision Guide)

| You want to record... | Use |
|------------------------|-----|
| An individual request's lifecycle | OTLP span (always on) |
| A business event (document updated, user invited) | `info` log line + span event |
| A counter (requests served, errors) | Prometheus counter (T1.21) |
| A latency distribution (DB query, search) | Prometheus histogram |
| A single DB query result | `debug` log + DB span attribute |
| A failed DB query | `error` log + span.RecordError + Prometheus error counter |
| An LLM call | OTLP span with token count attributes |
| A user-visible warning (e.g., deprecation) | `warn` log + response header |

The rule: **a fact should appear in exactly one channel**. If it's already
in a span attribute, don't log it. If it's already in a metric counter,
don't log every increment.

---

## Forbidden Patterns

- `fmt.Println(...)` anywhere outside `cmd/*/main.go` startup banners.
- `log.Println(...)` (stdlib logger; bypasses zap config and structured
  fields). Use `logger.Info(...)`.
- Logging raw `c.Request.Body` without `SanitizeBody`. Body may contain
  `apiKey` / `password` / tokens.
- Logging the full `error` chain as a single string with `%v`. Use
  `%w` to wrap, `errors.Is` / `errors.As` to inspect, and
  `span.RecordError(err)` so the OTLP path captures the structured chain.
- Logging inside a tight loop with per-iteration `info`. Either
  aggregate (one `info` per N iterations) or downgrade to `debug`.
- Logging `user_id: ""` instead of omitting the field. Empty values
  pollute filters; use `omitempty` or skip the field.
- "Pretty" log formatting (e.g., color, indentation) in any environment
  other than local dev. JSON is the production format.

---

## Examples

### Good: handler with logging + tracing

```go
func (h *DocumentHandler) Update(c *gin.Context) {
    ctx, span := otel.Tracer("doc.handler").Start(c.Request.Context(),
        "document.update",
        trace.WithAttributes(
            attribute.String("doc.code", c.Param("docCode")),
        ))
    defer span.End()

    log := h.log.With(
        "trace_id", trace.SpanContextFromContext(ctx).TraceID(),
        "request_id", c.GetString("request_id"),
        "component", "doc.handler",
        "doc_code", c.Param("docCode"),
    )

    var req UpdateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Warn("invalid request", "err", err.Error())
        span.RecordError(err)
        c.JSON(400, envelope.New("doc.bad_request", err.Error()))
        return
    }

    newVer, err := h.repo.UpdateWithCAS(ctx, req.DocCode, req.BaseVersion, req.Content)
    if err != nil {
        if errors.Is(err, doc.ErrVersionConflict) {
            log.Info("cas conflict", "base_version", req.BaseVersion)
            span.SetAttributes(attribute.Int64("doc.conflict", 1))
            c.JSON(409, envelope.New("doc.version_conflict", err.Error()))
            return
        }
        log.Error("update failed", "err", err.Error())
        span.RecordError(err)
        c.JSON(500, envelope.New("internal.error", ""))
        return
    }

    log.Info("document updated",
        "base_version", req.BaseVersion,
        "new_version", newVer,
        "duration_ms", time.Since(start).Milliseconds())
    c.JSON(200, envelope.Ok(...))
}
```

### Bad: same handler, no structure

```go
func (h *DocumentHandler) Update(c *gin.Context) {
    var req UpdateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Println("bad request:", err)               // ← unstructured
        c.JSON(400, gin.H{"error": err.Error()})       // ← wrong envelope
        return
    }
    newVer, err := h.repo.UpdateWithCAS(...)
    if err != nil {
        log.Println("update failed:", err)              // ← no trace correlation
        c.JSON(500, gin.H{"error": err.Error()})       // ← no envelope, no code
        return
    }
    fmt.Println("updated doc", req.DocCode)            // ← fmt.Println outside cmd/
    c.JSON(200, gin.H{"version": newVer})
}
```

The bad version loses: trace correlation, component name, structured fields,
envelope contract (T1.7), span attributes for metrics, and the `errors.Is`
check that distinguishes user-fixable (409) from internal (500) failures.

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §3.1 envelope, §4.5 dual-channel
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.1 AC-B8 (Langfuse) / AC-B10 (Prometheus)
- `.trellis/spec/backend/error-handling.md` — how errors map to envelope + status
- `backend/internal/middleware/logger.go` — `SanitizeBody` implementation
- `backend/internal/types/tracing.go` — `TracingContext` for async propagation