# Backend Quality Guidelines

> Code quality standards for backend Go code in `backend/`.

---

## Overview

Quality is enforced by **layered gates**. Each layer catches a different
class of defect:

| Gate | Tool | Catches |
|------|------|---------|
| 1 | `gofmt -s` + `goimports` | Style / formatting drift |
| 2 | `go vet ./...` | Suspicious constructs (printf args, lock copy, etc.) |
| 3 | `golangci-lint run` (`backend/.golangci.yml`) | Static analysis (errcheck, staticcheck, ineffassign, unused, gosimple, misspell, govet) |
| 4 | `go test -count=1 -short ./...` | Unit test failures + assertion flakiness |
| 5 | `go test -coverprofile` (core packages) | Coverage regression on `auth`, `rbac`, `doc`, `repository` (≥ 85%) |
| 6 | `go test -tags=integration ./test/integration/...` | Multi-component contract bugs |
| 7 | `make scrub-audit` | Vendor / internal-platform string leaks (ADR-007) |
| 8 | Trellis `trellis-check` sub-agent | Cross-package contract drift; spec compliance |

All eight run in CI for every PR; local `pre-commit` runs 1, 3, 5, 7
via `.pre-commit-config.yaml`. **No merge to `main` with any gate red.**

---

## Forbidden Patterns

These patterns trip one of the gates above OR have caused a real
incident. Each is a hard `REJECT` in code review.

### Error handling

- `//nolint:errcheck`, `//nolint:staticcheck` blanket annotations.
  Policy from `backend/.golangci.yml` header: **zero blanket `//nolint`
  suppressions**. If a linter rule conflicts with fork code, the rule is
  disabled globally (with comment) OR the call site is fixed.
- `_ = err` to discard an error outside `_test.go`. Even "obvious"
  errors (`defer f.Close()`) deserve a `defer func() { _ = f.Close() }()`
  with a reason comment if truly safe.
- `panic(err)` outside a startup-time preflight (`cmd/server/main.go`,
  `cmd/cli/main.go`, `cmd/migrate/main.go`). Domain code returns errors.
- `fmt.Errorf("...: %v", err)` (loses wrap). Use `fmt.Errorf("...: %w", err)`.

### Logging / I/O

- `fmt.Println(...)` outside `cmd/*/main.go`.
- `log.Println(...)` from `log` stdlib. Use `*zap.Logger`.
- `os.Exit(1)` inside `internal/`. The `internal/` packages do not own
  process lifecycle; they return errors and let `cmd/server/main.go` decide.
- Bare `http.Error(...)` from a handler. The handler writes an envelope.

### Concurrency

- Goroutines without an explicit lifecycle owner. Every `go func()`
  must have a documented owner: a context, a `sync.WaitGroup`, a worker
  pool with shutdown, or an asynq task with embedded `TracingContext`.
- Channel send without `select { case ch <- v: default: }` or
  `select { case ch <- v: <-ctx.Done(): }` in a long-lived goroutine —
  leaks block forever on shutdown.
- `sync.Mutex` copied by value (`mu := other.mu` is a data race even
  reading). Use `*sync.Mutex`.

### Database

- String concatenation into SQL text. Use `pgx.NamedArgs` or `$1, $2, ...`.
- `database/sql`-style `Query` + `Scan` for new code. New code uses
  `pgx v5` (`pgx.NamedArgs`, `pgx.RowToStructByName`).
- Migration that lacks a matching `.down.sql`.
- Migration that does NOT start with `BEGIN; ... COMMIT;` for
  multi-statement changes (Postgres doesn't auto-commit DDL).
- A commit that edits a committed migration. golang-migrate refuses
  re-run on checksum change; the right move is a new migration.
- DDL (`CREATE TABLE`, `ALTER TABLE`) inside repository code or
  startup hooks. All schema changes go through migrations.

### Types / API

- Exposing internal struct fields directly on the wire. The HTTP DTO
  is a separate struct (`CreateDocumentRequest`) from the domain
  model (`Document`); `c.ShouldBindJSON(&dto)` then map to domain.
- `interface{}` / `interface{}` return on a public API. Use
  `any` if Go 1.26+ (`interface{}` is identical but `any` reads cleaner).
- A package name with `_test` in the import path of a non-test file.

### Imports

- Wildcard imports (`import . "..."`).
- Dot imports outside generated test files.
- Importing a package only to get a constant — define the constant
  locally and reference the canonical source in a doc comment.

---

## Required Patterns

These are the patterns every new file in `internal/` should follow.

### Errors

- Every function that can fail returns `error` as the last value.
- Wrapped errors with context: `fmt.Errorf("repo document.Update id=%s: %w", id, err)`.
- Typed errors for known cases (`*VersionConflictError`), sentinel for
  matching (`ErrVersionConflict`).
- `errors.Is(err, doc.ErrVersionConflict)` over `strings.Contains(err.Error(), ...)`.

### Logging

- One `*zap.Logger` per package, initialized in `func initLogger(cfg)`
  or `New<Package>(cfg, log)`.
- Pass the logger as a dependency; never call `zap.L()` (the global).
- Every log line includes `component` and (when in request scope)
  `request_id` + `trace_id`.

### Context

- Every I/O function takes `ctx context.Context` as the first argument.
- A function that blocks on a remote call respects `ctx.Done()`.
- A function that fans out goroutines does `defer span.End()` and
  embeds `types.TracingContext` in any payload that crosses the
  request boundary (asynq tasks).

### Configuration

- All configuration via `internal/config` (viper-based). No
  `os.Getenv(...)` outside `internal/config`.
- Required env vars are validated at startup; missing required vars
  exit with a clear message + which env var is missing.

### Tests

- Test files live next to the source (`foo.go` ↔ `foo_test.go`).
- Table-driven tests for ≥ 3 cases.
- Use `t.Parallel()` only when the test is hermetic (no shared state).
- Mock external dependencies via small interfaces declared where they're
  consumed.
- Integration tests behind `//go:build integration` so `go test ./...`
  doesn't pull them in.

### Git

- One logical change per commit.
- Commit message format: `feat(backend): T1.x <title>` (matches the
  M1 plan). `<title>` ≤ 60 chars.
- No commits with `--no-verify` (skips pre-commit) except emergencies,
  documented in the PR description.

---

## Testing Requirements

### Coverage thresholds

Per `.trellis/tasks/10-09-M1-backend-cli/prd.md` AC-B9:

| Package | Threshold | Why |
|---------|-----------|-----|
| `internal/auth` | ≥ 85% | Security-critical; HMAC signature bugs are catastrophic |
| `internal/rbac` | ≥ 85% | Wrong deny = privilege escalation |
| `internal/doc` | ≥ 85% | docCode collision = data corruption; CAS bugs = lost writes |
| `internal/repository` | ≥ 80% (AC-B2) | Data-layer regressions are silent |
| Other `internal/*` | ≥ 70% | Industry baseline for business logic |
| `cmd/*` | No threshold (executable bootstrap) | Tests target the wiring, not main |

Coverage is checked via `go test -coverprofile=cov.out ./internal/auth ./internal/rbac ./internal/doc ./internal/repository && go tool cover -func=cov.out | tail -1` (per `implement.md` §3 validation).

### Test types

| Type | When | Tool |
|------|------|------|
| Unit | Every package; hermetic | `go test -short` |
| Integration | Cross-cutting (db + handler + http) | `go test -tags=integration ./test/integration/...` (gated by `INTEGRATION=1`) |
| BDD | User-visible workflows | godog specs in `test/bdd/` (lands M1) |
| Stress / race | Concurrency invariants (docCode, CAS) | `go test -race -count=100` |

### Required test cases per feature

- **HMAC sign/verify (T1.5):** ≥ 100 cases (AC-B5) including: valid
  signature, expired timestamp, future timestamp outside window, tampered
  body, replayed nonce, missing nonce, signature base mismatch, claim
  mismatch.
- **RBAC matrix (T1.6):** 40 cases (AC-B6) — 4 roles × 10 actions.
- **docCode generator (T1.3):** 1M iterations zero collision + 100-goroutine
  × 100K-iteration stress zero duplicate (AC-B3).
- **CAS (T1.4):** 100-goroutine same-doc contention — 1 success + 99
  `ErrVersionConflict` (AC-B4).
- **JWT / API Key capability:** every documented capability checked both
  positive and negative.

### Test data

- Test fixtures live in `testdata/` next to the test file (gitignored
  for binary, checked in for JSON/YAML fixtures).
- Use factory functions (`newTestDocument(t)`) not raw struct literals
  scattered across tests.

---

## Code Review Checklist

The reviewer verifies each box; the PR template (under
`.github/PULL_REQUEST_TEMPLATE.md` once T1.x lands) carries the same list.

### Spec compliance

- [ ] PR maps to a T1.x task in `.trellis/tasks/10-09-M1-backend-cli/implement.md`
      (or has a justifying ADR for new work).
- [ ] prd.md acceptance criteria for that T1.x are met (link in PR body).
- [ ] design.md contract referenced (envelope, CAS, sync-state, etc.) is
      honored.

### Code health

- [ ] `cd backend && golangci-lint run --config ./.golangci.yml ./...` exits 0.
- [ ] `cd backend && go vet ./...` exits 0.
- [ ] `cd backend && go test -count=1 -short ./...` exits 0.
- [ ] Coverage delta on `internal/auth`, `internal/rbac`, `internal/doc`,
      `internal/repository` not below threshold (85% / 80%).
- [ ] No new `//nolint` comments without an explanatory comment AND a
      tracking issue.
- [ ] No new TODOs without an issue link.

### Cross-layer

- [ ] Every new error code added to `internal/errors/envelope/codes.go`.
- [ ] Every new middleware added to `internal/server/router.go` in the
      documented order.
- [ ] Every new repository method has a typed error or returns one of
      the package's sentinels.
- [ ] Every new HTTP handler ends with `c.JSON(status, envelope.Ok(...)`
      or `c.JSON(status, envelope.FromError(err))` — no raw `gin.H`.
- [ ] Every new asynq payload embeds `types.TracingContext`.

### Hygiene

- [ ] No `fmt.Println`, no `log.Println` outside `cmd/*/main.go`.
- [ ] No `os.Getenv` outside `internal/config`.
- [ ] No `database/sql` (must use `pgx`).
- [ ] No silent error discards (`_ = err` outside tests).
- [ ] Commit message matches `feat(backend|cli): T1.x <title>` or
      `chore(backend): <title>`.

### Operational

- [ ] `make scrub-audit` exits 0 (no vendor / internal names in
      AI-KBS-authored files).
- [ ] Migration (if any) has both `.up.sql` and `.down.sql`, is
      idempotent (`IF NOT EXISTS` / `ON CONFLICT`), and is documented
      in the PR body.
- [ ] New env vars documented in `.env.example`.

---

## Common Mistakes (with examples)

### Mistake 1: blanket errcheck suppression

```go
// BAD — blanket suppression hides real bugs
//nolint:errcheck
json.Unmarshal(data, &cfg)
```

```go
// GOOD — explicit handling or documented reason
if err := json.Unmarshal(data, &cfg); err != nil {
    return fmt.Errorf("parse config: %w", err)
}
```

### Mistake 2: DTO → domain bleed

```go
// BAD — handler exposes repository struct
func (h *DocumentHandler) Get(c *gin.Context) {
    doc, _ := h.repo.Get(...)
    c.JSON(200, doc)   // ← internal fields leak (e.g., softDelete, internal notes)
}
```

```go
// GOOD — separate DTO
type DocumentResponse struct {
    DocCode   string    `json:"doc_code"`
    Title     string    `json:"title"`
    Version   int64     `json:"version"`
    UpdatedAt time.Time `json:"updated_at"`
}

func toResponse(d *repository.Document) *DocumentResponse { ... }
```

### Mistake 3: missing TracingContext on async payload

```go
// BAD — async task loses trace
type ReindexPayload struct {
    DocCode string
}

func NewReindexTask(p ReindexPayload) *asynq.Task { ... }
```

```go
// GOOD — async payload embeds TracingContext
type ReindexPayload struct {
    types.TracingContext
    DocCode string
}
```

### Mistake 4: silent CAS overwrite

```go
// BAD — drops base_version, breaks optimistic concurrency
func (r *DocumentRepo) Update(ctx context.Context, code string, content []byte) error {
    _, err := r.pool.Exec(ctx, "UPDATE documents SET content=$1 WHERE code=$2", content, code)
    return err
}
```

```go
// GOOD — uses CAS
func (r *DocumentRepo) Update(ctx context.Context, code string, base int64, content []byte) (int64, error) {
    tag, err := r.pool.Exec(ctx,
        "UPDATE documents SET content=$1, version=version+1 WHERE code=$2 AND version=$3",
        content, code, base)
    if err != nil { return 0, err }
    if tag.RowsAffected() == 0 {
        return 0, &doc.VersionConflictError{DocCode: code, BaseVersion: base}
    }
    return base + 1, nil
}
```

### Mistake 5: in-memory test that doesn't exercise the gate

```go
// BAD — passes unit, fails integration; mocks mask the bug
func TestDocumentRepo_Update(t *testing.T) {
    fake := &FakePool{tag: pgconn.CommandTag("1")}   // ← always says "1 row affected"
    err := (&DocumentRepo{pool: fake}).Update(ctx, "abc", 7, []byte("hi"))
    if err != nil { t.Fatal(err) }
}
```

```go
// GOOD — integration test against real pgxpool.Test; CAS conflict is observable
func TestDocumentRepo_Update_CASConflict(t *testing.T) {
    if testing.Short() { t.Skip("integration") }
    pool := pgxpool.NewTest(...)   // real Postgres
    repo := &DocumentRepo{ pool: pool }
    seed(t, pool, "abc", 7)
    _, err := repo.Update(ctx, "abc", 6, []byte("new"))   // wrong base_version
    if !errors.Is(err, doc.ErrVersionConflict) {
        t.Fatalf("expected ErrVersionConflict, got %v", err)
    }
}
```

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/implement.md` §3 validation commands, §6 review gates
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.1 AC-B2 / AC-B5 / AC-B6 / AC-B9
- `backend/.golangci.yml` — the linter policy this document codifies
- `.pre-commit-config.yaml` — local pre-commit hooks
- `Makefile` — `lint-backend`, `test-unit`, `test-integration`, `scrub-audit` targets
- `.trellis/spec/backend/error-handling.md` — error policy this complements
- `.trellis/spec/backend/logging-guidelines.md` — observability policy this complements