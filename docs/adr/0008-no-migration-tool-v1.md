# ADR-0008: Data-Migration Tool Deferred to v1.1

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

Some users of the internal system will want to migrate their existing data into AI-KBS. A migration tool is non-trivial (auth mapping, docCode assignment, attachment re-hosting, permission re-grant) and would consume 2-3 weeks of engineering.

## Decision

The v1.0.0 GA does **not** ship a data-migration tool. New users start from an empty workspace and `ai-kbs push` their documents. The migration tool ships in v1.1.

## Consequences

Easier:
- v1 ships on time
- We can validate the v1 architecture before investing in a migration path

Harder:
- Internal-system users must wait for v1.1 to migrate
- A small group of early adopters may not be willing to manually re-push

## Alternatives considered

1. **Ship migration in v1.** Rejected: 2-3 weeks of v1 timeline would slip.
2. **Outsource migration to a community script.** Rejected: the auth and permission semantics are too entangled to script externally.
