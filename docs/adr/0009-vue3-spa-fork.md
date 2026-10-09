# ADR-0009: Web UI Based on WeKnora Vue 3 SPA Fork + White-Label

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

The Web UI is a load-bearing part of v1 (see ADR-0006). Building from scratch would consume 30+ person-days; the WeKnora v0.7.1 Vue 3 SPA already provides a working implementation that is 90% structurally similar to what AI-KBS needs.

## Decision

Fork the WeKnora v0.7.1 Vue 3 SPA. Apply a white-label pass:

- Replace logo / favicon / brand color / top nav / footer / OG share image with AI-KBS assets
- Scrub i18n strings: remove any company-specific names, features, contact info
- Replace proprietary SSO (DingTalk / Feishu / WeCom) with standard OAuth2 (Google / GitHub / email+password) + JWT
- Realign API base path: `/api` → `/api/ai-kbs`
- Replace third-party SDKs: company-internal monitoring → Langfuse Web SDK; company CDN → jsDelivr / unpkg
- Strip `.env.example` / `vite.config.ts` / `package.json` of any company-specific flags / private npm registries

## Consequences

Easier:
- 7 person-days for the white-label + small customizations, vs. 30+ from scratch
- 5 core views (Workspace / Document / Ask / Spaces / Login) are testable on day one

Harder:
- We inherit WeKnora's frontend architecture decisions
- Upstream WeKnora frontend refactors require us to re-apply the white-label pass

## Alternatives considered

1. **Build the Web UI from scratch.** Rejected: 30+ person-days vs. 7.
2. **Adopt a headless CMS / Markdown viewer.** Rejected: lacks the live NDJSON streaming + RBAC surface we need.
