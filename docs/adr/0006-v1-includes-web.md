# ADR-0006: v1 GA Includes Web UI

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

A Web UI is the easiest way for non-CLI users (e.g. document reviewers, content managers) to interact with the knowledge base. A CLI-only v1 would narrow the audience significantly.

## Decision

The v1.0.0 GA includes a Web UI, forked from the WeKnora v0.7.1 Vue 3 SPA, white-labeled to "AI-KBS".

The Web UI covers 5 core views: Workspace, Document, Ask, Spaces, Login. The investment is ~7 person-days; one experienced Vue engineer can complete it.

## Consequences

Easier:
- The product is usable for non-technical users from day one
- We avoid a "Web UI coming soon" gap that erodes trust

Harder:
- One more codebase to keep current (frontend + backend)
- White-labeling must be done carefully to avoid leaking WeKnora's brand

## Alternatives considered

1. **Ship CLI + API only in v1.** Rejected: the Web UI is a load-bearing part of the value proposition.
2. **Build the Web UI from scratch.** Rejected: 7 person-days vs. an estimated 30+ days from scratch.
