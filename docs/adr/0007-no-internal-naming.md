# ADR-0007: No Upstream-Internal Naming in the Public Codebase

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

The team has built a similar system internally. Some of its naming, command structure, and configuration conventions are platform-specific and must not appear in the public open-source codebase.

## Decision

The AI-KBS codebase must not contain any [internal platform] name, command, or configuration as an actual implementation. The CI runs a scrub-audit step that fails the build on a hit.

The conceptual borrowings (Workspace, Space, docCode, CAS, version chain, workspace/bind sync, rollback-as-revert) are explicitly allowed; they are general engineering patterns and not platform-specific.

## Consequences

Easier:
- The codebase is verifiably platform-neutral
- Onboarding an engineer who is unfamiliar with the internal system is friction-free

Harder:
- We must avoid letting any LLM-assisted commit leak a platform name (hence the CI scrub)

## Alternatives considered

1. **Allow some leakage.** Rejected: the risk of a partial leak is worse than a strict no-leak policy.
2. **Build a transformer that strips platform names from copy-pasted code.** Considered; deferred to v2.
