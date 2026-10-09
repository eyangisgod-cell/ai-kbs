# Backend Development Guidelines

> Best practices for backend Go development in this project.

---

## Overview

This directory contains guidelines for backend development in `backend/`
(the only Go module). All files describe **AI-KBS conventions** (not the
WeKnora fork's current shape, since M1 will rewrite the fork to AI-KBS
patterns). Sub-agents writing M1 / M2 / M3 code MUST follow these.

When you change a backend file, re-read the matching guideline first —
it codifies a contract that may have evolved since the fork import.

---

## Guidelines Index

| Guide | Description | Status |
|---------|-------------|--------|
| [Directory Structure](./directory-structure.md) | `cmd/`, `internal/`, `migrations/`, `client/`, layered import graph | Filled (M1 unblock) |
| [Database Guidelines](./database-guidelines.md) | pgx v5 patterns, migrations policy, idempotency | Filled (T0.8 + M1 unblock) |
| [Error Handling](./error-handling.md) | Envelope v1.0 contract, typed errors, namespaces | Filled (M1 unblock) |
| [Quality Guidelines](./quality-guidelines.md) | Forbidden patterns, coverage, review checklist | Filled (M1 unblock) |
| [Logging Guidelines](./logging-guidelines.md) | zap + OTLP dual-channel, scrub policy | Filled (M1 unblock) |

---

## Cross-Reference Map

```
directory-structure.md  ──►  design.md §1 module topology, §2 module boundaries
database-guidelines.md ──►  golang-migrate v4.19.1, M1 AC-B1
error-handling.md     ──►  design.md §3.1 envelope, §3.4 CLI parse, §4.3 namespaces
quality-guidelines.md ──►  backend/.golangci.yml (v2 schema), M1 AC-B2/B5/B6/B9
logging-guidelines.md ──►  design.md §4.5 dual-channel, internal/middleware/logger.go
```

When a guideline changes, update the matching design.md or ADR so
behavior and policy stay aligned.

---

## How To Use These Guidelines

For each T1.x / T2.x implementation:

1. Read the PRD (`prd.md`) and design (`design.md`) for the task.
2. Read the **matching guideline** in this directory.
3. When in doubt about a convention, the guideline wins over generic
   Go advice; the project wins over the guideline when explicitly
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
Sub-agents writing M1 / M2 / M3 code MUST honor these contracts; treat
deviations as code-review rejections.