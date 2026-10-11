# ADR-0001: Fork WeKnora v0.7.1 as the Backend Base

- **Status:** Accepted
- **Date:** 2026-10-02
- **Deciders:** jimmy, project lead

## Context

We need a backend for an open-source AI knowledge base. The team has previously built a similar system internally; ~80% of the work overlaps with what we'd need to write from scratch: RAG, ReAct Agent, Wiki Mode, RBAC, MCP, Langfuse. WeKnora v0.7.1 (Apache 2.0) is a public open-source implementation that covers the same surface.

Building from scratch would consume 8+ months of engineering time and re-invent the wheel on RAG orchestration, agent tool registration, vector store integration, and audit logging. Forking lets us focus on the ~20% that is genuinely new: three-layer retrieval (ES + RAG + SAG), CAS versioning, workspace/sync, MCP tool surface.

## Decision

Fork WeKnora v0.7.1 as the backend base. Adapt (do not adopt verbatim):

- Replace the internal auth module with HMAC + JWT + scoped API Key
- Replace the internal storage backend with a pluggable adapter (Local / MinIO / S3 / OSS / TOS / KS3 / OBS)
- Replace the internal Agent tool registry with our own tool set
- Replace the internal frontend with a fork of the WeKnora Vue 3 SPA, white-labeled to "AI-KBS"
- Re-route the API base path to `/api/ai-kbs/`

The fork is **not a git submodule** — it is a direct copy with a separate maintenance policy. We will track upstream changes via diff-and-rebase, not via automatic pulls.

## Consequences

Easier:
- ~80% reduction in baseline engineering work
- A known-good RAG and ReAct implementation on day one
- Apache 2.0 license, no commercial entanglements

Harder:
- We inherit WeKnora's code-style and architectural assumptions
- We are now responsible for keeping the fork current with upstream bug fixes
- Some WeKnora-internal abstractions will need to be unwound over time

## Alternatives considered

1. **Build from scratch.** Rejected: 8+ months for ~80% overlap; we would be reinventing RAG orchestration.
2. **Use WeKnora as a git submodule.** Rejected: too easy to drift from upstream; harder to apply systematic modifications.
3. **Adopt a different OSS base (e.g. Dify, FastGPT).** Rejected: those are more SaaS-oriented and have weaker CAS / version-chain semantics.
