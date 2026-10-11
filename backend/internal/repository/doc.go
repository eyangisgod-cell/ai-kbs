// Package repository implements the AI-KBS data-access layer for the 13
// spec-compliant tables introduced by migration 000079_ai_kbs_core_schema.
//
// It is intentionally HTTP-agnostic and has no business logic: each
// repository exposes pure CRUD + existence checks over pgx v5. Handlers
// translate repository errors into the envelope exactly once at the edge.
//
// # Naming fork-compat (chunks_ai / audit_logs_ai)
//
// The upstream WeKnora fork already owns SQL tables named `chunks` and
// `audit_logs`. To stay additive — and to avoid a destructive rename that
// would fork the schema_migrations checksum history — migration 000079
// creates the AI-KBS versions as `chunks_ai` and `audit_logs_ai`. The
// application domain stays clean: the Go domain types are `Chunk` and
// `AuditLog`. The table-name alias lives here, in this package, behind the
// `TableChunks` / `TableAuditLogs` constants. SQL references these
// constants, never the table-name strings, so a future rename (or a switch
// to a non-forked upstream) is a single-line edit.
//
// This is documented as a decision in
// `.trellis/tasks/10-09-M1-backend-cli/design.md` §6 (fork-compat) and in
// the 000079 migration header comment.
//
// # pgx v5
//
// Repositories accept a small `DBTX` interface (Exec / Query / QueryRow +
// transactional variants) that is satisfied by both `*pgxpool.Pool` and
// `pgx.Tx`. The constructor variants accept either, which keeps the unit
// tests pure-mock while letting production code compose a pool or a
// per-request transaction seamlessly. See `errors.go` and the per-repo
// files for the contract.
package repository