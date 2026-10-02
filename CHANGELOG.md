# Changelog

All notable changes to **ai-kbs** are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **T0.1** — Initial repository scaffold for AI-KBS v1 MVP:
  - `backend/` forked from [WeKnora v0.7.1](https://github.com/Tencent/WeKnora) (Apache 2.0); module renamed to `github.com/eyangisgod-cell/ai-kbs`; all internal `github.com/Tencent/WeKnora` import paths rewritten (1,139 files) including Go source, proto, Python packaging, and selected Vue frontend files.
  - Placeholder directories with `.gitkeep`: `cli/`, `mcp-server/`, `sag-bridge/`, `docreader/`, `frontend/`, `helm/`, `docs/`.
  - Root files: `LICENSE` (Apache 2.0), `README.md` (中英双语), `CHANGELOG.md`, `.gitignore` (workspace `*.bak`, OS / IDE / Node / Go / Python build artifacts).
- **Scrub audit** — `grep`-based check ensures zero occurrences of internal platform names (`losu`, `losu-wiki`, `losu-ai`, `losu-wiki-cli`) in source / docs / config; CI will fail otherwise.
- **T0.10** — Frontend forked from WeKnora v0.7.1 Vue 3 SPA into `frontend/`:
  - `package.json`: `name` → `ai-kbs-frontend`, `version` → `1.0.0`. No private npm registry URLs.
  - `vite.config.ts`: added `/mcp` proxy entry (WebSocket-enabled for SSE) + preserved vendor chunk splitting (`vendor-mermaid` / `vendor-markdown` / `vendor-highlight`).
  - `tsconfig.app.json`: added path aliases `@components/*`, `@views/*`, `@stores/*`, `@api/*` alongside existing `@/*`.
  - `index.html`: `<title>` + meta tags updated to AI-KBS product positioning.
  - `.eslintrc.cjs` (Vue 3 + TS + Prettier), `.env.example` (`VITE_AIKBS_API_BASE=/api/ai-kbs`, etc.), AI-KBS-branded `README.md`.
  - **Verification**: `npm install` (379 packages in ~4 min), `npm run build` (`✓ built in 24.78s` → `dist/` produced), `npm run dev` (Vite 7.3.6 ready in 488 ms → HTTP 200 on `:5173`). All checks performed in a throw-away build copy on F: drive (see "Build environment" below).
  - White-labeling of brand colors / logo / favicon / i18n strings / SSO SDK removal is owned by **T1.23** (per design.md §2.8.2).

### Fixed (post-merge)

- **T0.10 follow-up** — `trellis-check` found 25 `github.com/Tencent/WeKnora` references that the T0.10 scrub missed. Replaced all visible URLs with `github.com/eyangisgod-cell/ai-kbs` across 10 frontend files (Login.vue header logo + info link, UserMenu.vue docs + GitHub entries, ApiIntegrationSettings.vue, IntegrationSettingsSection.vue IM guide, GraphSettings.vue, ModelSettings.vue, SystemInfo.vue migration troubleshooting + bug-report issue, chunkingSamples.ts clone snippet), `backend/cmd/desktop/update.go` release-check URL, and `backend/docs/swagger.json` contact URL. Replaced 14 backend Go comment "Tencent/WeKnora#NNNN" upstream-issue references with "upstream WeKnora#NNNN" (no code logic changed) across `internal/datasource/*`, `internal/application/service/datasource_*.go`, and `cli/internal/compat/compat.go`. `go build ./internal/...` exit=0.

### Known Build Constraints (T0.1 follow-ups)

- `backend/internal/container/container.go` unconditionally imports `github.com/asg017/sqlite-vec-go-bindings/cgo` and `_ "github.com/duckdb/duckdb-go/v2"`. These pull in C source / prebuilt `.a` archives that require CGO_ENABLED=1 + a C compiler (gcc / clang) on every platform. Consequence:
  - On **Linux / macOS dev** (gcc or clang installed): `go build ./...` succeeds.
  - On **Windows without gcc in PATH** (current T0.1 dev box): only `./internal/...` builds. `cmd/server` and `cmd/desktop` fail until the imports are gated by `//go:build sqlite` / replaced with the pure-Go `ncruces` variant. Tracked as T0.1.1 follow-up (1-line patch + `go build -tags=...` test).
- Resolution will land in **T1.2 (Backend skeleton)**, where we already plan to fork `internal/container/container.go` for ai-kbs's PostgreSQL-first configuration (PostgreSQL is the v1 default per PRD §3.1; SQLite is dev-only).

### Build environment

- **T0.10 install/build verification** was performed in a throw-away copy at `F:/ai-kbs-frontend-build/` because the project's primary drive `E:` is an **exFAT volume at 98 % capacity** (≈ 4.4 GB free). The `tdesign-icons-vue-next` package alone emits thousands of per-icon `.d.ts` files; unpacking on `E:` triggers `ENOSPC` (verified by two retry attempts). `F:` is NTFS with ~ 320 GB free, so the build copy lives there until `npm install` completes successfully on the source location.
- NTFS junctions / directory symlinks across `exFAT (E:) → NTFS (F:)` are blocked by the Windows kernel, so the working-copy approach is the only currently-available option. Source files on `E:/my-project/my-open-source/ai-kbs/frontend/` remain clean — no `node_modules/` was extracted in place. Re-run `cd frontend && npm install` after freeing ≥ 3 GB on `E:` (or relocating the project to an NTFS volume).

### Planned (not in this release)

- v1.0.0 GA — Backend + CLI + MCP + Web UI + Helm (Week 10–11, milestone M3).

[Unreleased]: https://github.com/eyangisgod-cell/ai-kbs
