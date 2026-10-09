# Frontend Development Guidelines

> Best practices for frontend Vue 3 + TS development in this project.

---

## Overview

This directory contains guidelines for `frontend/` (Vue 3 + Vite 5 +
Pinia 2 + Vue Router 4 + TypeScript, package manager pnpm). All files
describe **AI-KBS conventions** (not the WeKnora fork's current shape,
since M1 will scrub / rewrite the frontend to AI-KBS patterns).

Sub-agents writing M1 / M2 / M3 frontend code MUST follow these.

---

## Guidelines Index

| Guide | Description | Status |
|---------|-------------|--------|
| [Directory Structure](./directory-structure.md) | `src/{api,components,composables,stores,router,views,types,utils}/` layered graph | Filled (M1 unblock) |
| [Component Guidelines](./component-guidelines.md) | `<script setup lang="ts">`, props/emits, tokens, a11y | Filled (M1 unblock) |
| [Hook Guidelines](./hook-guidelines.md) | Composables (`use*`), data-fetching contract | Filled (M1 unblock) |
| [State Management](./state-management.md) | Pinia 2 composition, server state, persistence | Filled (M1 unblock) |
| [Quality Guidelines](./quality-guidelines.md) | Forbidden patterns, bundle budget, review checklist | Filled (M1 unblock) |
| [Type Safety](./type-safety.md) | Strict TS, Zod boundary, branded IDs | Filled (M1 unblock) |

---

## Cross-Reference Map

```
directory-structure.md ──►  design.md §1 frontend box (api → stores → router)
component-guidelines.md ──►  design.md §1, §4.6 state persistence + white-label tokens
state-management.md   ──►  design.md §4.6 token + workspace persistence
hook-guidelines.md    ──►  directory-structure.md composables/ + state-management.md
type-safety.md        ──►  design.md §3.1 envelope v1.0 (mirrored as types/envelope.ts)
quality-guidelines.md ──►  prd.md §3.3 AC-F1–F5 + bundle NFR-FE-01..05
```

When a guideline changes, update the matching design.md or ADR so
behavior and policy stay aligned.

---

## How To Use These Guidelines

For each T1.x / T2.x implementation:

1. Read the PRD (`prd.md`) and design (`design.md`) for the task.
2. Read the **matching guideline** in this directory.
3. When in doubt about a convention, the guideline wins over generic
   Vue / TS advice; the project wins over the guideline when explicitly
   justified in code review.

---

## Living Style Conventions

These guidelines are not aspirational — they describe the **current**
project state. If you discover a real exception while implementing,
update the guideline in the same PR so future sub-agents see the same
reality.

**Language**: All documentation in English.

---

## Filled By

Bootstrap task `00-bootstrap-guidelines` (T0.x → 2026-10-09).
Sub-agents writing M1 / M2 / M3 frontend code MUST honor these
contracts; treat deviations as code-review rejections.

## Activation Notes

- `.eslintrc.cjs` ships but is **dormant** (T1.22 activates).
- `vitest` lands as components ship (T1.22 follow-up).
- `playwright` lands with M2 E2E coverage.