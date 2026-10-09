# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

<!--
Document your project's database conventions here.

Questions to answer:
- What ORM/query library do you use?
- How are migrations managed?
- What are the naming conventions for tables/columns?
- How do you handle transactions?
-->

(To be filled by the team)

---

## Query Patterns

<!-- How should queries be written? Batch operations? -->

(To be filled by the team)

---

## Migrations

> T0.8 introduced the migration framework. Source of truth: `golang-migrate/migrate/v4` (v4.19.1).

### Storage layout

| DB | Migrations dir | Notes |
|----|----------------|-------|
| PostgreSQL (prod / default) | `backend/migrations/versioned/` | Each file is `NNNNNN_name.{up,down}.sql`; numbered continuously across all migrations. |
| SQLite (tests / local-only) | `backend/migrations/sqlite/` | Fork-specific subset; do not edit casually. |
| MySQL / ParadeDB | `backend/migrations/{mysql,paradedb}/` | Out of scope for AI-KBS v1 — left from upstream fork for reference. |

The schema_migrations table is owned by golang-migrate. Do not write to it manually — use the CLI.

### Auto-run on server startup

`internal/database.RunMigrationsWithOptions(dsn, opts)` runs at server boot. Options:

- `AutoRecoverDirty bool` — if `true`, the server force-rolls back the partial version and retries the migration when it lands in a dirty state. Default is `false` in the AI-KBS path because the upstream fork left `AutoRecoverDirty=false` in `RunMigrations`; prefer fixing dirty state explicitly via `make migrate-force` so the operator sees the failure.

### Operator CLI (T0.8)

```bash
make migrate-up                              # apply all pending
make migrate-down steps=1                    # roll back one
make migrate-status                          # print version + dirty
make migrate-force version=7                 # force schema_migrations to 7
```

All four wrap `backend/cmd/migrate/main.go`, which uses the same `golang-migrate` library as the server. DSN defaults to `$AIKBS_DB_URL`. Override migrations source with `-migrations file://path`.

### Naming conventions

| Element | Convention | Example |
|---------|------------|---------|
| File | `NNNNNN_snake_case.up.sql` + matching `.down.sql` | `000012_add_doc_code_index.down.sql` |
| Number | Monotonic, zero-padded 6 digits; never reuse a number | `000013_…`, `000014_…` |
| Identifier | snake_case, prefer long descriptive names over abbreviations | `embedding_tag_id` not `emb_tid` |

### Authoring rules

1. **Every migration MUST have both `.up.sql` and `.down.sql`.** golang-migrate requires both; missing `.down.sql` makes rollback impossible.
2. **Never edit a committed migration.** Add a new migration that supersedes the old one. golang-migrate refuses to re-run a migration whose checksum has changed.
3. **Prefer additive changes.** A column add is safe; a `DROP COLUMN` is rarely safe because old app versions may still read it. For destructive changes, deprecate → migrate data → drop in a later release.
4. **Idempotent where possible.** Use `CREATE INDEX IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `ON CONFLICT DO NOTHING`. This makes `AutoRecoverDirty=true` recovery work cleanly.
5. **Long statements belong in their own migration.** If a single statement exceeds ~200 lines (typical for backfills), split it across multiple migrations to keep each one reviewable.
6. **No DDL in application code.** All schema changes go through migrations, never through `CREATE TABLE` in repos or `ALTER TABLE` in startup hooks.

### Dirty-state recovery

A "dirty" schema_migrations row means a migration failed partway. The server refuses to start. Recovery:

```bash
make migrate-status    # confirm dirty + version
# Inspect the database for partial state. Either:
#   (a) complete the failed migration manually, then:
make migrate-force version=<last clean version>
#   (b) roll back the partial state, then:
make migrate-down steps=1
make migrate-up
```

`AutoRecoverDirty=true` is only safe when the migration is idempotent (see rule 4).

---

## Naming Conventions

<!-- Table names, column names, index names -->

(To be filled by the team)

---

## Common Mistakes

<!-- Database-related mistakes your team has made -->

(To be filled by the team)
