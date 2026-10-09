# Backend Directory Structure

> How Go code is organized in `backend/` (the only Go module in this repo).

---

## Overview

The Go module name is `github.com/eyangisgod-cell/ai-kbs` (see
`backend/go.mod`). The module ships **one executable per `cmd/*` directory**
and a layered set of `internal/` packages. The `internal/` prefix means
nothing outside this module can import these packages — that's the rule
that keeps the cross-tier contracts enforceable.

The layout follows three principles:

1. **Layered boundaries** — handlers depend on repositories via interfaces;
   repositories never know about HTTP. Errors flow up; types flow down.
2. **One responsibility per package** — `internal/auth` is auth, not auth+users.
   If a package name reads like a category ("helpers", "common"), it's wrong.
3. **Migrations live next to the code** — `backend/migrations/` is part of
   the module, not a sidecar. The CI lint config and `make migrate-*`
   targets treat it as such.

---

## Directory Layout

```
backend/
├── go.mod                                # module github.com/eyangisgod-cell/ai-kbs (go 1.26)
├── go.sum
├── .golangci.yml                         # per-module lint config (the canonical one)
├── .gitignore                            # per-module ignores (allow !.golangci.yml at root)
│
├── cmd/                                  # one directory per executable
│   ├── server/                           # the main HTTP server (T1.7+ boots here)
│   ├── cli/                              # CLI binary (T1.10–T1.20) — built to ./bin/clai
│   ├── migrate/                          # T0.8 operator CLI around golang-migrate
│   ├── download/                         # fork binary; T1.x CGO follow-up (excluded from lint)
│   ├── desktop/                          # fork binary; ditto
│   └── duckdb/                           # fork binary; CGO-only, excluded from lint
│
├── internal/                             # non-importable from outside this module
│   ├── auth/                             # T1.5: HMAC + JWT + API Key middleware
│   │   ├── hmac.go                       # sign / verify; ±300s window; nonce via Redis SETNX
│   │   ├── hmac_test.go                  # ≥ 100 cases per AC-B5
│   │   ├── jwt.go                        # RS256 + capability claims
│   │   ├── apikey.go                     # keyId.keySecret, capability-bounded
│   │   └── middleware.go                 # mounts all three on Gin
│   ├── rbac/                             # T1.6: (role, resource, action) → bool
│   │   ├── matrix.go                     # 4 roles × 10 actions
│   │   ├── matrix_test.go                # 40 cases per AC-B6
│   │   └── middleware.go                 # 403 on denial
│   ├── doc/                              # T1.3 + T1.4: docCode generator + CAS
│   │   ├── doc_code.go                   # 8-char base62; 1M/0-collisions + 100-goroutine stress
│   │   ├── cas.go                        # UpdateWithCAS(ctx, code, baseVersion, newContent)
│   │   └── errors.go                     # ErrVersionConflict, ErrNotFound
│   ├── repository/                       # T1.2: 13 repositories
│   │   ├── document.go                   # pgx v5; receives ctx; returns domain model
│   │   ├── binding.go
│   │   ├── user.go
│   │   ├── ...
│   │   └── *_test.go                     # ≥ 80% per AC-B2
│   ├── database/                         # T0.8 + T1.1
│   │   ├── connect.go                    # pgx pool init
│   │   ├── migration.go                  # RunMigrationsWithOptions(dsn, opts)
│   │   └── migration_test.go
│   ├── observe/                          # T1.8 + T1.21
│   │   ├── otlp.go                       # OTLP/HTTP → Langfuse
│   │   ├── metrics.go                    # Prometheus 6 categories
│   │   └── logger.go                     # zap factory
│   ├── errors/                           # T1.7 envelope owner (frozen)
│   │   └── envelope/
│   │       ├── envelope.go               # Envelope struct, Ok/New constructors
│   │       ├── codes.go                  # canonical HTTP status mapping
│   │       ├── typed.go                  # CodedError interface
│   │       └── translate.go              # FromError — used by handlers
│   ├── sync/                             # T1.18–T1.20
│   │   ├── state.go                      # sync-state.json schema v1.0
│   │   ├── pull.go                       # NFR-PERF-04: 1000 files < 60s
│   │   └── push.go                       # 4-strategy CAS resolution
│   ├── cli/                              # T1.10–T1.17: per-subcommand orchestration
│   │   ├── login.go, init.go, status.go, bind.go, unbind.go
│   │   ├── read.go, write.go, edit.go, version.go, move.go, resolve.go
│   │   ├── output/                       # envelope parse + text/json/ndjson rendering
│   │   │   ├── machine.go                # Parse(r io.Reader)
│   │   │   ├── text.go                   # RenderHuman(env)
│   │   │   └── exitcode.go               # CLI exit code table
│   │   └── workspace/                    # local file ops + sync-state I/O
│   ├── types/                            # cross-package struct definitions
│   │   ├── tracing.go                    # TracingContext — async trace propagation
│   │   └── envelope.go                   # re-exports of envelope core types
│   ├── handler/                          # HTTP entry points (Gin)
│   │   ├── document.go                   # GET/POST/PUT/DELETE for /api/ai-kbs/documents/:code
│   │   ├── binding.go
│   │   ├── auth.go
│   │   └── middleware/                   # request id, CORS, body-size, recovery, tracing, logger
│   ├── service/                          # business logic that orchestrates repositories
│   ├── event/                            # in-process event bus (fork carry-over)
│   ├── agent/                            # fork agent runtime; rewritten in M2
│   ├── application/                      # fork use-case layer; rewritten in M2
│   ├── types/                            # cross-cutting domain types
│   ├── utils/                            # fork helpers; excluded from lint (CGO)
│   ├── container/                        # DI wiring (fork carry-over)
│   └── config/                           # viper-based config loading
│
├── migrations/                           # part of the module
│   ├── versioned/                        # PostgreSQL — NNNNNN_name.{up,down}.sql
│   │   ├── 000000_init.up.sql
│   │   ├── 000000_init.down.sql
│   │   └── ...
│   ├── sqlite/                           # SQLite — fork-specific subset
│   ├── mysql/                            # reference only (out of scope v1)
│   └── paradedb/                         # reference only (out of scope v1)
│
├── client/                               # the Go SDK (importable; lives outside internal/)
│   ├── knowledgebase.go, knowledge.go, ...
│   ├── auth.go
│   └── model.go
│
├── docreader/                            # proto + parser; fork carry-over
├── cli/                                  # fork acceptance suite (separate go.mod soon)
├── test/                                 # integration + BDD tests
│   ├── integration/                      # go test -tags=integration ./test/integration/...
│   └── bdd/                              # godog specs (T1.x fills in)
│
├── config/                               # static YAMLs + prompt_templates/
├── scripts/                              # operational scripts (cloud-image, systemd)
│
├── bin/                                  # build output (gitignored)
└── dist/                                 # release artifacts (gitignored)
```

---

## Module Organization Rules

### `cmd/*` — entry points only

Each `cmd/<name>/main.go` is a thin wrapper:

```go
package main

import (
    "github.com/eyangisgod-cell/ai-kbs/internal/server"   // ← wire + boot
)

func main() {
    if err := server.Run(); err != nil {
        log.Fatal(err)
    }
}
```

`cmd/` MUST NOT contain business logic. Configuration loading lives in
`internal/config`; HTTP wiring lives in `internal/handler`; startup
orchestration lives in `internal/server`.

### `internal/<area>/` — domain packages

| Package | Knows about | Does NOT know about |
|---------|-------------|---------------------|
| `internal/repository/*` | DB schema, pgx, SQL | HTTP, JSON, business rules |
| `internal/auth` | HMAC/JWT/APIKey, Gin context (writing user) | DB writes (except `api_key` table) |
| `internal/rbac` | role/resource/action matrix | Business semantics |
| `internal/doc` | docCode, CAS | I/O (except DB uniqueness check) |
| `internal/observe` | OTLP, Prometheus, zap | Business span field names |
| `internal/errors/envelope` | Frozen envelope contract | Business errors |
| `internal/sync` | sync-state.json, pull/push orchestration | User-visible output (delegated to `output`) |
| `internal/output` | Envelope parse, text/json/ndjson render | Where errors come from |
| `internal/cli/*` | Cobra command binding, flag parsing | Direct DB / LLM (always via envelope) |
| `internal/handler/*` | Gin routes, request decoding | Business rules (delegated to `service`) |

The rule for new packages: **if its name fits a noun that the team
already understands** (auth, doc, binding, sync), put it in `internal/`.
**If the name sounds like a category** ("helpers", "common", "utils"),
split it by responsibility until each package has one.

### `internal/handler/middleware/` — Gin middleware

- One file per middleware (`logger.go`, `tracing.go`, `cors.go`,
  `request_id.go`, `recovery.go`, `body_size.go`).
- Each is independently testable; integration tests in `handler_test.go`.
- Order matters — registration in `internal/server/router.go`:

```go
r.Use(middleware.RequestID)        // 1. mint request_id
r.Use(middleware.Tracing)          // 2. open OTLP span
r.Use(middleware.Recovery)         // 3. panic catcher (outermost span)
r.Use(middleware.Logger)           // 4. request log line
r.Use(middleware.BodySize(8<<20))  // 5. 8 MiB cap
r.Use(cors.Default())              // 6. CORS
```

### `migrations/` — schema as code

Per `.trellis/spec/backend/database-guidelines.md`:

- `versioned/` for PostgreSQL (prod). Files `NNNNNN_name.{up,down}.sql`.
- `sqlite/` for SQLite (tests + dev-only). Fork-specific subset; do not
  edit casually.
- `mysql/` and `paradedb/` reference only (out of scope v1).

The directory is part of the module so `make migrate-up` and the server
boot path can find it via `file://migrations/versioned` (default).

---

## Naming Conventions

### File names

| Element | Convention | Example |
|---------|------------|---------|
| Go file | snake_case.go | `doc_code.go`, `sync_state.go` |
| Test file | `<source>_test.go` | `cas_test.go` next to `cas.go` |
| Generated | `<name>_gen.go` or `zz_generated_*.go` | `mocks_gen.go` |
| Mocks | `<interface>_mock.go` | `repository_mock.go` |
| Migration | `NNNNNN_<snake_name>.up.sql` + `.down.sql` | `000013_add_doc_code_index.up.sql` |

### Package names

| Element | Convention | Example |
|---------|------------|---------|
| Domain package | one short noun, no underscores | `auth`, `rbac`, `doc` |
| Sub-area | parent + dot | `errors/envelope`, `cli/output` |
| Avoid | `utils`, `helpers`, `common`, `misc` | — |

### Identifier names

| Element | Convention | Example |
|---------|------------|---------|
| Exported type | PascalCase | `DocumentRepository` |
| Unexported helper | camelCase | `parseRequestID` |
| Constant | PascalCase (NOT SCREAMING) | `ErrVersionConflict`, `DefaultTimeout` |
| Interface (single method) | `-er` suffix | `Code() string` ⇒ no name; the interface is implicit if used as a value |
| Interface (multi method) | noun phrase | `CodedError`, `DocumentReader` |
| Error variable | `Err<Name>` | `ErrVersionConflict`, `ErrNotFound` |
| Error type | `<Name>Error` | `VersionConflictError`, `NotFoundError` |

### Identifier content

- Use full words; avoid abbreviations: `repository` not `repo`, `document`
  not `doc`. (`doc` is allowed in package names per the table above, but
  not in identifiers.)
- DocCodes themselves use the `aB3xY7zQ` style (8-char base62).

---

## Cross-Package Import Graph (must not cycle)

```
cmd/*     →  internal/server  →  internal/handler  →  internal/service
                                ↑                  ↘
                                │                   internal/repository
                                │                   ↑
                          internal/auth            internal/database
                          internal/rbac            (pgx pool, migrate)
                                ↓
                          internal/doc (CAS, docCode)
                          internal/errors/envelope  ← handlers translate to this
                          internal/observe          ← handlers + services record on
                          internal/sync             ← CLI sync subcommands
                          internal/cli/output       ← CLI all subcommands

internal/client (SDK) is importable; it uses HTTP, NOT internal/ directly.
```

No `internal/<a>` may import `cmd/<b>`. No `internal/handler` may import
`internal/repository` except through an interface declared in
`internal/handler`. The interface lives where it's **consumed**, not
where it's **implemented** (Go-idiomatic).

---

## Where to Put New Code

| You're adding... | Put it in |
|------------------|-----------|
| a new HTTP endpoint | `internal/handler/<area>.go` + register in `internal/server/router.go` |
| a new SQL query | new method on existing repository in `internal/repository/<area>.go` |
| a new domain type | `internal/types/<area>.go` |
| a new error code | `internal/errors/envelope/codes.go` (extend mapping table) |
| a new middleware | `internal/handler/middleware/<name>.go` |
| a new CLI subcommand | `cmd/cli/cmd/<area>/<sub>.go` + Cobra binding in `cmd/cli/main.go` |
| a new migration | `migrations/versioned/NNNNNN_<name>.{up,down}.sql` |
| a new LLM provider | `internal/agent/llm/<provider>.go` (M2) |
| a new search backend | `internal/search/<backend>.go` (M2) |
| a new MCP tool | `internal/sag/tools/<tool>.go` (M2) |
| a new SDK method | `internal/catalog/<area>.go` |

---

## Examples

- A well-organized handler module: `internal/handler/document.go` — single
  file per resource, multiple methods (List/Get/Create/Update/Delete).
- A well-organized repository module: `internal/repository/document.go` —
  one file per domain table, methods return `(*Document, error)`,
  errors are `repository.ErrNotFound` or richer typed.
- A well-organized error extension: adding `doc.code_malformed` →
  `internal/doc/errors.go` (typed), `internal/errors/envelope/codes.go`
  (mapping), `internal/errors/envelope/translate.go` (CLI exit code).
- A well-organized CLI subcommand: `internal/cli/login.go` orchestrates
  `internal/cli/output` (for envelope render) and `internal/auth`
  (for token fetch); never touches DB or HTTP client directly.

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 module topology, §2 module boundaries
- `.trellis/spec/backend/database-guidelines.md` — migrations layout
- `.trellis/spec/backend/error-handling.md` — `internal/errors/envelope` placement
- `.trellis/spec/backend/logging-guidelines.md` — `internal/observe` placement
- `backend/go.mod` — module path + dependency surface