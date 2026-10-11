# CI/CD Workflow Thinking Guide

> Lessons learned from T0.2 (CI/CD pipeline) and every GitHub Actions task since.
> Use this when **creating or modifying** any file under `.github/workflows/`.

---

## Why This Guide Exists

CI workflows are written once and run thousands of times. A subtle bug (wrong
action version, shared artifact name, missing cache key) can cause green CI
that hides real failures, or red CI on every PR until someone manually fixes
it. Most CI bugs are silent — the workflow runs, succeeds, but doesn't
actually do what you think.

This guide collects the gotchas that bit us. **Skim it before authoring any
new workflow file.**

---

## Pre-Flight Checklist (run before committing a workflow change)

- [ ] **Action versions are pinned to a major tag** (e.g., `@v4`, `@v5`). Never `@latest` or `@main`.
- [ ] **golangci-lint binary version matches the config schema version.** If `backend/.golangci.yml` declares `version: "2"`, the workflow MUST use the `golangci-lint-action` v2.x binary (`version: v2.x`). v1.x binary silently fails to parse v2 schema configs.
- [ ] **Every `setup-*` action has a cache key bound to a dependency file**, not a branch. `setup-go` → `cache-dependency-path: backend/go.sum`. `setup-python` → `cache: 'pip'` + `cache-dependency-path: **/requirements*.txt` (or `pyproject.toml` for poetry/uv). `setup-node` → `cache: 'pnpm'` + `path: frontend/pnpm-lock.yaml`.
- [ ] **Workflows skip gracefully when their target module is absent.** Use `if: hashFiles('<module-glob>') != ''` with ALL dependency files in the guard (e.g., for pnpm: BOTH `package.json` AND `pnpm-lock.yaml`). A partial guard causes install steps to fail with confusing errors on PRs.
- [ ] **Concurrency groups with `cancel-in-progress: true`** for PR/push workflows. NEVER for `release.yml` — cancelling a half-published release is worse than wasted runner minutes.
- [ ] **Minimal `permissions:` block at the workflow or job level.** Default `contents: read`; explicitly upgrade to `contents: write` only in release/publish jobs.
- [ ] **No secret echoes.** No `${{ secrets.* }}` interpolated into `run:` blocks without masking. No `env:` blocks that bake a token into shell history.

---

## Gotcha: Shared Artifact Name in Matrix Jobs Collides on Merge

`actions/upload-artifact@v4` with `merge-multiple: true` **overwrites** files
with the same name across matrix legs. Five of six SHA256 sums silently lost.

**Wrong**:
```yaml
strategy:
  matrix: { os: [linux, darwin, windows], arch: [amd64, arm64] }
steps:
  - run: sha256sum bin > SHA256SUMS.txt
  - uses: actions/upload-artifact@v4
    with:
      name: sums            # ← all 6 legs collide on this name
      path: SHA256SUMS.txt
```

**Correct**: each leg writes to a unique name; a later job concatenates.

```yaml
# per-leg
- run: sha256sum bin > SHA256SUMS-${{ matrix.os }}-${{ matrix.arch }}.txt
- uses: actions/upload-artifact@v4
  with:
    name: sums-${{ matrix.os }}-${{ matrix.arch }}
    path: SHA256SUMS-${{ matrix.os }}-${{ matrix.arch }}.txt

# publish-release job
- uses: actions/download-artifact@v4
  with:
    pattern: sums-*
    path: sums/
    merge-multiple: true
- run: cat sums/SHA256SUMS-*.txt > SHA256SUMS.txt
```

---

## Gotcha: `pnpm install --frozen-lockfile` Without Lockfile Fails the Job

A `hashFiles('package.json') != ''` guard is not enough — pnpm with a frozen
lockfile requires the lockfile to exist. Either guard on BOTH, or use
`--no-frozen-lockfile` for the bootstrap case.

**Wrong**:
```yaml
- if: hashFiles('frontend/package.json') != ''
- run: pnpm install --frozen-lockfile   # ← fails; no pnpm-lock.yaml yet
```

**Correct**:
```yaml
- if: hashFiles('frontend/package.json') != '' && hashFiles('frontend/pnpm-lock.yaml') != ''
- run: pnpm install --frozen-lockfile
```

For the bootstrap case (lockfile not yet committed), use a separate workflow
or a job-level `continue-on-error: true` plus an explicit "scaffold expected
in T1.22" annotation so future readers know it's intentional.

---

## Gotcha: `actions/checkout@v4` Doesn't Resolve Tag in `workflow_dispatch`

When `release.yml` allows `workflow_dispatch` with a tag input, the default
checkout ref is the default branch — NOT the specified tag. The release ends
up tagged correctly but binaries are built from the wrong ref.

**Pattern**: branch on `github.event_name`:

```yaml
- uses: actions/checkout@v4
  with:
    ref: ${{ github.event_name == 'workflow_dispatch' && inputs.tag || github.ref }}
```

For tag-push triggers, `github.ref` already resolves correctly — no override
needed. The branch is only required for the dispatch path.

---

## Gotcha: Matrix `fail-fast: true` Hides Partial Failures

If your 6-platform matrix has `fail-fast: true` and leg 3 fails, the runner
cancels the other 5 legs. You never see whether they would have succeeded —
hard to debug a platform-specific issue.

**Default**: `fail-fast: false` for any matrix with 4+ legs. Cancellation
saves ~30s per leg but costs you diagnostic data.

---

## Gotcha: `${{ github.event.pull_request.head.sha }}` Is Empty on `push`

PR-targeting jobs that reference head SHA get empty strings when the trigger
is a plain `push` (e.g., direct push to main). Either gate the expression on
the trigger type or use `${{ github.sha }}` for both.

```yaml
- name: Build
  env:
    SHA: ${{ github.event_name == 'pull_request' && github.event.pull_request.head.sha || github.sha }}
```

---

## Pattern: Layered Jobs Over Single Monolithic Job

When one workflow lints backend + python + frontend, do NOT use a single job.
A failure in any one cancels the others (wasted diagnostics), and you can't
mark one as required-blocking vs. advisory.

**Pattern**:
```yaml
jobs:
  lint-go:        { needs: setup; runs-on: ubuntu-latest; ... }   # blocking
  lint-python:    { needs: setup; runs-on: ubuntu-latest; ... }   # blocking
  lint-frontend:  { needs: setup; runs-on: ubuntu-latest; continue-on-error: true }   # advisory
```

`continue-on-error: true` is the right tool for "fail-loud-but-don't-block"
during early development, before all sub-modules are scaffolded.

---

## Pattern: 6-Platform Matrix (NFR-COMPAT-01)

```yaml
strategy:
  fail-fast: false
  matrix:
    include:
      - { os: ubuntu,   arch: amd64,  suffix: '' }
      - { os: ubuntu,   arch: arm64,  suffix: '' }
      - { os: macos,    arch: amd64,  suffix: '' }
      - { os: macos,    arch: arm64,  suffix: '' }
      - { os: windows,  arch: amd64,  suffix: '.exe' }
      - { os: windows,  arch: arm64,  suffix: '.exe' }
runs-on: ${{ matrix.os }}-${{ matrix.arch }}
env:
  CGO_ENABLED: 0       # ← CRITICAL: backend has cgo/stub split; non-CGO build is the v1 baseline
  GOFLAGS: -trimpath
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with:
      go-version: '1.23'
      cache-dependency-path: backend/go.sum
  - run: |
      cd backend
      go build -trimpath -ldflags "-s -w -X main.version=${{ github.sha }}" \
        -o ../dist/ai-kbs-${{ matrix.os }}-${{ matrix.arch }}${{ matrix.suffix }} \
        ./cmd/server
```

`CGO_ENABLED=0` is non-negotiable until every CGO-only dep has a tagged stub
sibling (the backend cgo/stub split from `8d2a8b5` made this safe).

---

## Pattern: Tag-Triggered Release With Draft/Prerelease Mode

```yaml
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release (e.g., v1.0.0)'
        required: true

jobs:
  build:
    # ... matrix as above ...

  publish-release:
    needs: build
    runs-on: ubuntu-latest
    permissions: { contents: write }
    steps:
      - uses: actions/download-artifact@v4
        with: { pattern: 'aikbs-*', merge-multiple: true }
      - name: Resolve release mode from tag
        id: mode
        run: |
          case "${{ github.ref_name }}" in
            *-test)    echo "draft=true"     >> "$GITHUB_OUTPUT" ;;
            *-rc*|*-alpha*|*-beta*)
                          echo "prerelease=true" >> "$GITHUB_OUTPUT" ;;
            *)          echo "draft=false"    >> "$GITHUB_OUTPUT" ;;
          esac
      - uses: softprops/action-gh-release@v2
        with:
          draft: ${{ steps.mode.outputs.draft }}
          prerelease: ${{ steps.mode.outputs.prerelease }}
          files: dist/*
          generate_release_notes: true
```

`v0.1.0-test` → draft; `v1.0.0-rc.1` → prerelease; `v1.0.0` → stable.

---

## When NOT to Add a Workflow

- The repo has **no code in that language yet** (e.g., writing `rust.yml` when
  there is no `Cargo.toml`). Add it when the scaffold lands; gating on
  `hashFiles` works but creates dead config.
- The job runs only **once** (e.g., a one-shot migration script). Use a
  `workflow_dispatch` job or a Makefile target instead.

---

## Related

- T0.2 (CI/CD 流水线) — initial 4-workflow scaffold
- T0.5 (代码规范工具配置) — golangci-lint v2 schema baseline
- `.golangci.yml` at repo root + per-module configs
