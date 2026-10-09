# ADR-0004: CAS + docCode Instead of Git for Versioning

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

Documents need fine-grained versioning with audit, partial rollback, and per-document RBAC. Git-style versioning is overkill for individual documents and has poor fit for non-technical users.

## Decision

Use a **CAS (compare-and-set) optimistic locking** model with `base_version` checks. Each document has a stable 8-character `docCode` (base62) that survives rename / move. Versions form an append-only chain; rollback produces a new version on top of the current one, never deleting intermediate versions.

## Consequences

Easier:
- Per-document permissions, audit, and rollback are first-class
- Conflict resolution is well-defined (`keep-mine` / `keep-theirs` / `both` / `auto`)

Harder:
- No Git-style branching or three-way merge (deferred to v2)
- No LFS-style binary diffs (we store full content per version, compressed)

## Alternatives considered

1. **Store documents in Git.** Rejected: perms, audit, and the LFS story are awkward.
2. **Use event sourcing (no version field on documents).** Rejected: harder to reason about, harder to support "list versions".
