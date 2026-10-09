# Error Handling

> How errors flow from repository → handler → client in AI-KBS.

---

## Overview

AI-KBS uses a **frozen envelope contract** for every API response,
anchored at `internal/errors/envelope` (T1.7). All errors are **named**
(`namespace.subname`), **typed** at the package boundary, and **translated**
exactly once — at the handler edge — into the envelope.

The flow:

```
repository ─┐
internal/doc ┤── typed Go errors ──┐
internal/auth┤                     │
            ↓                     ↓
         handler ──────→ translate ─→ envelope v1.0 ─→ JSON ─→ client
                              ↑
                span.RecordError + log.Error("component", ...)
```

Three rules enforce this:

1. **Repositories return typed Go errors** (`doc.ErrVersionConflict`,
   `repository.ErrNotFound`, `auth.ErrInvalidCredentials`). They never
   return envelope-shaped structs — that's a handler concern.
2. **Handlers translate exactly once**. A handler that wraps an error
   in `envelope.New(...)` MUST be the last code on the path before
   `c.JSON(...)`. No helper in between may re-wrap.
3. **Middleware (auth / rbac) returns typed errors** with a `.Status()`
   method that maps to an HTTP code, which the handler reads. Middleware
   never builds the envelope itself.

---

## Envelope v1.0 (frozen)

Frozen per `.trellis/tasks/10-09-M1-backend-cli/design.md` §3.1. Adding
fields is a breaking change; extension goes in `meta` (sub-object, optional).

```go
// internal/errors/envelope/envelope.go
type Envelope struct {
    Success   bool        `json:"success"`
    ExitCode  int64       `json:"exit_code"`
    Data      any         `json:"data"`
    Error     *ErrDetail  `json:"error"`
    RequestID string      `json:"request_id"`           // ULID
    TraceID   string      `json:"trace_id,omitempty"`   // W3C, OTLP path only
    Meta      map[string]any `json:"meta,omitempty"`    // frozen since v1.0
}

type ErrDetail struct {
    Code    string `json:"code"`           // "namespace.subname"
    Message string `json:"message"`        // human-readable, may be empty
}
```

### Invariants

- `success == false` ⇒ `error.code` set, `data == null`, `exit_code != 0`
- `success == true` ⇒ `error == null`, `exit_code == 0`, `data` set
- `request_id` always present (minted by `middleware.RequestID` if absent)
- `trace_id` present iff the request is on the OTLP path
- CLI exit code = `envelope.ExitCode`; non-zero on `success == false`

---

## Error Code Namespaces

Per `design.md` §4.3, every error code is `namespace.subname`. The namespace
is owned by exactly one package; new codes are added by editing the
namespace owner's `codes.go`.

| Namespace | Owner | HTTP code range | Examples |
|-----------|-------|-----------------|----------|
| `auth.*` | `internal/auth` | 401, 403 | `auth.login.fail`, `auth.token.expired`, `auth.apikey.revoked` |
| `rbac.*` | `internal/rbac` | 403 | `rbac.permission_denied`, `rbac.role_mismatch` |
| `doc.*` | `internal/doc` | 404, 409, 422 | `doc.not_found`, `doc.version_conflict`, `doc.code_malformed` |
| `binding.*` | `internal/binding` | 400, 404 | `binding.not_found`, `binding.duplicate` |
| `sync.*` | `internal/sync` | 207, 409, 423 | `sync.partial_fail`, `sync.locked`, `sync.cas_unresolved` |
| `search.*` | `internal/search` (M2) | 502, 504 | `search.upstream.timeout`, `search.empty` |
| `sag.*` | `internal/sag` (M2) | 502, 504 | `sag.runtime.unavailable` |
| `internal.*` | server core | 500 | `internal.error`, `internal.db_unavailable`, `internal.timeout` |

### HTTP mapping (default)

The mapping is centralized in `internal/errors/envelope.MapStatus(code)`
so handler code reads `envelope.MapStatus(err.Code())`, not the status
constant directly:

```go
// canonical mapping
"auth.login.fail"          → 401
"auth.token.expired"       → 401
"auth.apikey.revoked"      → 401
"rbac.permission_denied"   → 403
"doc.not_found"            → 404
"doc.version_conflict"     → 409
"doc.code_malformed"       → 422
"binding.duplicate"        → 409
"sync.partial_fail"        → 207    // multi-status (T1.19 pull AC-C10)
"sync.cas_unresolved"      → 409    // push AC-C11 exit 24
"internal.error"           → 500
"internal.timeout"         → 504
```

A new error code MUST add an entry to this table in the same commit.

### Naming

- Lowercase, ASCII, dot-segmented. **No spaces, no underscores.**
- One frozen subname per concept. If the meaning changes, add a new
  subname rather than overloading.
- `data` (the success payload) is NOT part of the error code; it's the
  envelope's top-level `data` field.

---

## Typed Errors at Boundaries

Each package that produces errors owns a small set of sentinel values
with structured detail:

```go
// internal/doc/cas.go
var (
    ErrVersionConflict = errors.New("version conflict")
    ErrNotFound        = errors.New("document not found")
)

type VersionConflictError struct {
    DocCode       string
    BaseVersion   int64
    CurrentVersion int64
}

func (e *VersionConflictError) Error() string  { return "version conflict" }
func (e *VersionConflictError) Is(target error) bool { return target == ErrVersionConflict }
func (e *VersionConflictError) Code() string    { return "doc.version_conflict" }
func (e *VersionConflictError) Status() int     { return 409 }
```

The interface that all translateable errors satisfy:

```go
// internal/errors/typed.go
type CodedError interface {
    error
    Code() string    // e.g. "doc.version_conflict"
    Status() int     // HTTP status code
}
```

Callers use `errors.Is(err, doc.ErrVersionConflict)` to inspect; the
handler switches on the sentinel then unwraps with `errors.As` to read
structured detail for `meta`:

```go
var vce *doc.VersionConflictError
if errors.As(err, &vce) {
    envelope.WithMeta(c, map[string]any{
        "current_version": vce.CurrentVersion,
        "base_version":    vce.BaseVersion,
    })
}
```

---

## Error Propagation

### Repository layer

- Returns CodedError values for known cases.
- Returns wrapped errors with `fmt.Errorf("repo document: %w", err)` for
  unexpected errors — the wrap chain is the forensic record.
- Never returns HTTP / envelope types.
- Never logs the error; the call site (or the handler) decides.

### Service / business logic layer

- Receives repository errors.
- Translates domain-specific failures into richer typed errors
  (`doc.VersionConflictError` rather than `errors.New("conflict")`).
- For internal failures, wraps with `fmt.Errorf("component action: %w", err)`
  and lets the handler decide what to surface.

### Middleware

- Returns CodedError to the handler via `c.Error(err)`.
- Never builds an envelope — middleware cannot know `data`.
- Never logs at `error` for rejected requests (the request was *not*
  executed); logs at `warn` with the rejection reason.

### Handler

- The single point that calls `envelope.New(...)` or `envelope.Ok(...)`.
- Inspects via `errors.Is` / `errors.As`.
- Calls `span.RecordError(err)` for any non-success path.
- Logs at `error` for 5xx, `info` for 4xx (user-explicit failures), `debug`
  for 2xx (already covered by the request span).

---

## `internal/errors` Package (Owner)

The package owns:

| File | Responsibility |
|------|----------------|
| `envelope.go` | `Envelope`, `ErrDetail`, `Ok(...)`, `New(code, msg)` constructors |
| `codes.go` | The HTTP status mapping table (canonical source) |
| `typed.go` | `CodedError` interface, `errors.As` helpers |
| `translate.go` | `FromError(err) *Envelope` — single translator used by handlers |

Handlers MUST use `envelope.FromError(err)` rather than rolling their own
status mapping. The translator:

1. If `err == nil` ⇒ `Ok(data)`.
2. If `errors.As(err, &CodedError)` ⇒ read `.Code()` + `.Status()`, build envelope.
3. If `errors.Is(err, context.DeadlineExceeded)` ⇒ `internal.timeout` (504).
4. Otherwise ⇒ `internal.error` (500), log the original wrapped chain
   (server-side) but DO NOT include detail in `error.message`.

### Anti-pattern: leaking internal detail

```go
// BAD — error.message leaks SQL state to client
c.JSON(500, envelope.New("internal.error",
    "pq: duplicate key value violates unique constraint \"idx_doc_code\""))
```

```go
// GOOD — envelope message is generic; detail is in logs + span
log.Error("create document failed", "err", err.Error())
span.RecordError(err)
c.JSON(500, envelope.New("internal.error", "an internal error occurred"))
```

The client gets a request_id; the operator greps logs by request_id to
see the SQL state. Nothing leaves the server that helps them attack it.

---

## CLI Translation

`internal/output/machine.go` parses the envelope exactly once:

```go
func Parse(r io.Reader) (*envelope.Envelope, error)
```

CLI exit codes mirror `envelope.ExitCode`:

| Envelope | CLI exit code |
|----------|---------------|
| `success=true` | 0 |
| `success=false`, code in `auth.*`, `internal.*` | 1 |
| `success=false`, code `doc.version_conflict` | 22 |
| `success=false`, code `sync.partial_fail` | 23 |
| `success=false`, code `sync.cas_unresolved` | 24 |

`internal/output/exitcode.go` owns this table.

---

## Common Mistakes

1. **Returning `gin.H{"error": ...}` instead of envelope.** Hand-rolled
   error shapes miss `request_id`, `exit_code`, `trace_id`, and the
   `error.code` namespace. CI's `make lint` will not catch this; only
   the cross-package contract test does.
2. **Mapping status codes in the handler.** `c.JSON(409, ...)` couples
   the handler to HTTP semantics. The envelope package owns the mapping;
   handlers call `envelope.FromError(err)`.
3. **Logging the same error at every layer.** One `log.Error` at the
   handler edge, with the original wrapped chain. Repository code does
   not log.
4. **`errors.New("conflict")` without a typed value.** Callers can't
   `errors.Is` on a string; use a sentinel (`ErrVersionConflict`) and a
   typed struct (`*VersionConflictError`) for detail.
5. **Returning `error` from a middleware-built goroutine.** Async errors
   cross the request boundary; they need `TracingContext` embedded
   (see `logging-guidelines.md`).
6. **Stacking wraps without context.** `fmt.Errorf("%w", err)` alone
   discards the layer info: prefer
   `fmt.Errorf("repository document.Update: %w", err)`.

---

## Examples

### Good: handler with translation

```go
func (h *DocumentHandler) Update(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "document.update")
    defer span.End()

    var req UpdateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, envelope.FromError(
            fmt.Errorf("invalid JSON: %w", err)))
        return
    }

    newVer, err := h.repo.UpdateWithCAS(ctx, req.DocCode, req.BaseVersion, req.Content)
    if err != nil {
        span.RecordError(err)
        logFromCtx(ctx).Error("update failed",
            "doc_code", req.DocCode,
            "err", err.Error())
        c.JSON(envelope.FromError(err).StatusCodeOr500(),
            envelope.FromError(err))
        return
    }

    c.JSON(200, envelope.Ok(map[string]any{"new_version": newVer}))
}
```

### Bad: handler with hand-rolled errors

```go
func (h *DocumentHandler) Update(c *gin.Context) {
    var req UpdateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})   // ← no envelope, no code, no request_id
        return
    }
    newVer, err := h.repo.UpdateWithCAS(...)
    if err != nil {
        if strings.Contains(err.Error(), "conflict") {  // ← string match on err
            c.JSON(409, gin.H{"error": "version conflict"})
        } else {
            c.JSON(500, gin.H{"error": err.Error()})     // ← leaks internal detail
        }
        return
    }
    c.JSON(200, gin.H{"new_version": newVer})           // ← success without envelope
}
```

The bad version has: no envelope contract, no namespace codes, no
request_id, no trace correlation, no typed sentinel, and SQL/internal
detail in the response body.

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §3.1 envelope, §3.4 CLI parse, §4.3 namespaces
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.1 AC-B7 (envelope required)
- `.trellis/spec/backend/logging-guidelines.md` — how errors are recorded on the OTLP span + log line
- `.trellis/spec/backend/directory-structure.md` — where `internal/errors/envelope/` lives
- `backend/internal/middleware/logger.go` — `SanitizeBody` (request body never logged raw)