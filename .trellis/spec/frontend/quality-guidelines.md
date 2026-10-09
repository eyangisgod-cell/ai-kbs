# Frontend Quality Guidelines

> Code quality standards for `frontend/` (Vue 3 + Vite 5 + TS + Pinia).

---

## Overview

Frontend quality is enforced by **layered gates**:

| Gate | Tool | Catches |
|------|------|---------|
| 1 | `tsc --noEmit` (strict mode) | Type errors |
| 2 | `eslint .` (T1.22 activates; `.eslintrc.cjs` currently dormant) | Style / unused vars / accessibility hint |
| 3 | `npm run lint` (alias for `eslint .`) | Same as 2 |
| 4 | `npm run test` (Vitest unit, when added) | Logic regressions |
| 5 | `npm run test:e2e` (Playwright, planned) | User-flow regressions |
| 6 | `npm run build` | Bundle errors + chunk-size warnings |
| 7 | `npm run build:analyze` | Bundle-size budget (NFR-FE-01: first-screen < 200KB gzipped) |
| 8 | `make scrub-audit` | Vendor / internal-platform string leaks (ADR-007) |

Per `.eslintrc.cjs`, the ESLint config is **dormant in v1** — it
ships but is not invoked by `make lint-frontend` until T1.22 enables
it. Once activated, `lint-frontend` becomes a required gate.

The browserslist target (AC-F3) is:

```
Chrome >= 100, Firefox >= 100, Safari >= 15, Edge >= 100
```

Set in `package.json` under `"browserslist"`. Vite uses this for
autoprefixer and Babel/ESBuild targets.

---

## Forbidden Patterns

These patterns are explicit code-review rejections in M1. Some are
already caught by `tsc` strict mode; the rest are caught by ESLint or
the code-review checklist.

### Type system

- `any` (use `unknown` + Zod, or a precise type).
- `as any`, `as unknown as X` double-cast (use Zod or fix the upstream type).
- `// @ts-expect-error` / `// @ts-ignore` (fix the underlying type).
- Non-null assertion `!` (use `assertDefined(x)` from `@/utils/assertDefined`,
  or guard explicitly).
- `JSON.parse(s)` without a schema (wrap in `Schema.parse(JSON.parse(s))`).

### Reactivity

- `reactive(...)` for primitives or small objects (use `ref` — `reactive`
  proxies a plain object, which loses ref-scalar semantics and is harder
  to refactor).
- Direct DOM mutation outside a directive.
- `setTimeout` / `setInterval` in `<script setup>` without `onUnmounted`
  cleanup.

### Components

- Two-way binding via `v-model` on props not declared as `update:<propName>`
  emit (use `:value` + `@input` or declare the prop as a writable v-model).
- Inline `style="..."` with hardcoded colors / px (use scoped CSS + tokens).
- `<div onclick="...">` (use `<button>` or `<a>` + a real handler).
- Class-based components (Vue 3 SFC only; no `Vue.extend`).

### Imports / modules

- `import * as Foo from 'foo'` (use named imports).
- Cyclic imports between `views/` and `components/` (the layered import
  graph in `directory-structure.md` is non-cyclic on purpose).
- Importing from `node_modules/` absolute paths (`import x from '/node_modules/x'`).
- `require()` in TS / Vue files.

### State / API

- `localStorage.setItem(...)` outside `pinia-plugin-persistedstate`
  configuration (the plugin is the policy layer; ad-hoc writes bypass
  the contract).
- `fetch(...)` in a component (use `src/api/*`).
- Direct `axios` import in a component or composable (use `src/api/http.ts`).

### Errors

- `console.log` (use the `useToast()` UI store for user-visible; for
  debug use `import.meta.env.DEV` guard).
- Empty `catch {}` swallowing (log + toast or rethrow).
- Returning raw `error.message` strings to UI (route through `ErrorEnvelope`).

### Misc

- Comments that say "TODO" without an issue link.
- `git commit --no-verify` (bypasses pre-commit hooks).
- Vendor / fork-internal strings (`钉钉`, `飞书`, `企微`, `咕噜`, `[内部 Wiki 系统]`,
  `legacy.weknora`, etc.) — caught by `make scrub-audit`. Fork code
  under `frontend/src/` is exempt per ADR-007; new AI-KBS-authored
  files are not.

---

## Required Patterns

### Type safety

- TypeScript strict mode (`strict: true`, `noUncheckedIndexedAccess: true`).
- `tsconfig.json` includes `"noUncheckedIndexedAccess": true` so `arr[i]`
  is typed `T | undefined` and forces a check.
- Zod-validated boundary at `src/api/*`.
- Branded IDs (`DocCode`, `UserId`, `BindingId`, `Version`).

### Reactivity

- `ref<T>()` for state, `computed` for derived.
- `shallowRef<T>()` for large read-only lists (avoids deep proxy overhead).
- `defineModel` or explicit `defineProps + defineEmits<update:prop>` for
  v-model on props.

### Components

- `<script setup lang="ts">` only.
- `<template>` has one root element unless intentionally a fragment.
- `<style scoped>` only (no global CSS unless adding tokens).
- Tokens via CSS custom properties (`var(--kb-...)`); no hardcoded colors.

### Accessibility

- Semantic HTML (`<button>`, `<main>`, `<nav>`, `<article>`).
- ARIA only when needed (`aria-label`, `aria-live`, `role`).
- Keyboard handlers paired with mouse handlers; visible focus ring.
- Color contrast ≥ WCAG AA (4.5:1 body text, 3:1 large text).
- Form fields have a real `<label>` (not placeholder-only).

### State

- Pinia stores in composition style.
- Persistent fields via `pinia-plugin-persistedstate` with explicit
  `paths` (NEVER `persist: true` without `paths`).
- Server data flows: HTTP → api/* → store → component. Never component → HTTP.

### Errors

- All HTTP errors normalize to `ErrorEnvelope` (via `toErrorEnvelope(e)`).
- For user-visible failures, call `useUiStore().pushToast({ ... })`.
- For background, log at `warn` level via `console.warn` (dev) or
  forward to the backend logger.

### Git

- One logical change per commit.
- Commit message format: `feat(frontend): T1.x <title>` (matches M1 plan).
- No `--no-verify`; no force-push to `main`.

---

## Testing Requirements

### Coverage (M1 → V1 target)

| Category | Threshold | Notes |
|----------|-----------|-------|
| Composables | ≥ 80% | Pure logic; easy to test |
| Utils | ≥ 90% | Pure helpers; pair `.ts` with `.test.mjs` |
| Pinia stores | ≥ 70% | Tests cover actions + selected getters |
| Components (unit) | ≥ 60% | Snapshot + props/emits behavior |
| End-to-end (Playwright, M2+) | MVP critical paths only | login, doc CRUD, agent chat (M3) |

M1 ships with **Vitest as a stub** (the `package.json` adds it; tests
land as components ship). The threshold ramp-up:

- M1 closeout: 0% — no tests yet, just the harness.
- M2 closeout: utils ≥ 90%, composables ≥ 80%, others TBD.
- M3 / v1.0.0 GA: targets above are enforced.

### Test types

| Type | When | Tool |
|------|------|------|
| Unit | utils, composables, pure store logic | Vitest (`*.test.mjs` or `*.spec.ts`) |
| Component | One component in isolation; props / emits / a11y | `@vue/test-utils` + Vitest; `vitest-axe` for a11y |
| Integration | Store + api (mocked HTTP) | MSW (Mock Service Worker) + Vitest |
| End-to-end | Critical user flows (M2+) | Playwright |
| Visual regression | Optional, design-system components | Percy / Playwright screenshot diff |

### Required test cases per feature (M2+)

- **Login flow**: invalid creds → toast, valid creds → redirect, expired
  token → re-login prompt.
- **Knowledge CRUD**: create with validation errors, edit with CAS
  conflict → toast + refetch.
- **Chat send**: optimistic insert, success update, network error → rollback.
- **Agent run start**: loading state, success → agent list, error → toast.

### Component tests

```ts
// src/components/knowledge-base/KnowledgeBaseListItem.spec.ts
import { mount } from '@vue/test-utils'
import { axe } from 'vitest-axe'
import KnowledgeBaseListItem from './KnowledgeBaseListItem.vue'

describe('KnowledgeBaseListItem', () => {
  it('renders the title', () => {
    const wrapper = mount(KnowledgeBaseListItem, { props: { kb: mockKb } })
    expect(wrapper.text()).toContain(mockKb.name)
  })

  it('emits select when title clicked', async () => {
    const wrapper = mount(KnowledgeBaseListItem, { props: { kb: mockKb } })
    await wrapper.get('[data-test="title"]').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([mockKb.id])
  })

  it('has no axe violations', async () => {
    const wrapper = mount(KnowledgeBaseListItem, { props: { kb: mockKb } })
    const results = await axe(wrapper.element)
    expect(results).toHaveNoViolations()
  })
})
```

---

## Bundle & Performance

Per `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3:

| NFR | Target | Tool |
|-----|--------|------|
| NFR-FE-01 | First-screen chunk < 200KB gzipped | `npm run build:analyze` (rollup-plugin-visualizer) |
| NFR-FE-02 | Largest chunk < 500KB gzipped | same |
| NFR-FE-03 | Time-to-interactive < 3s on 4G | Lighthouse CI |
| NFR-FE-04 | p95 input latency < 100ms | web-vitals in dev |
| NFR-FE-05 | Browserslist target (see above) | `.browserslistrc` + `npm run lint` |

### Bundle rules

- Lazy-load every route (`defineAsyncComponent` + dynamic `import()`).
- Lazy-load heavy libraries (`monaco-editor`, `echarts`, `pdfjs`) on
  first use.
- No `import * from 'lodash'` — use `lodash-es` per-method imports.
- Tree-shake icons (`unplugin-icons` with auto-import).

---

## Code Review Checklist

The reviewer verifies each box; the PR template carries the same list.

### Spec compliance

- [ ] PR maps to a T1.x in M1 implementation plan
      (or has a justifying ADR for new work).
- [ ] prd.md acceptance criteria for that T1.x are met (link in PR body).
- [ ] design.md contract referenced is honored (api → store → component).

### Type & lint

- [ ] `cd frontend && npm run type-check` exits 0.
- [ ] `cd frontend && npm run lint` exits 0 (once T1.22 activates ESLint).
- [ ] No new `any`, no new `as any`, no `// @ts-expect-error`.
- [ ] No new hardcoded colors or px literals in templates.

### Component

- [ ] Component has typed `defineProps` (no `any`).
- [ ] Component has typed `defineEmits` (verb past-tense names).
- [ ] One root in `<template>` (or intentional fragment with `key`).
- [ ] No business logic in `<script setup>` beyond local UI state.
- [ ] No `fetch` / `axios` import in the component.
- [ ] Keyboard activation for any clickable non-`<button>` element.

### State

- [ ] All HTTP calls go through `src/api/<resource>.ts`.
- [ ] All server state lives in a Pinia store.
- [ ] Persistent fields use `pinia-plugin-persistedstate` with explicit `paths`.
- [ ] No `localStorage.setItem` outside the persistence plugin.
- [ ] Store actions have `try/catch/finally` with `ErrorEnvelope`.

### Accessibility

- [ ] Semantic HTML elements used.
- [ ] All form fields have a `<label>`.
- [ ] Color contrast meets WCAG AA.
- [ ] Visible focus state (default browser ring OR `:focus-visible`).
- [ ] `vitest-axe` passes (if component test added).

### Bundle

- [ ] `npm run build` exits 0.
- [ ] `npm run build:analyze` shows first-screen chunk < 200KB gzipped.
- [ ] No new heavy import at top of `main.ts`.

### Hygiene

- [ ] No `console.log` outside `import.meta.env.DEV` guards.
- [ ] No `// TODO` without an issue link.
- [ ] No vendor / internal names in AI-KBS-authored files
      (`make scrub-audit` clean).
- [ ] Commit message matches `feat(frontend): T1.x <title>` or
      `chore(frontend): <title>`.

---

## Common Mistakes (with examples)

### Mistake 1: any in a public signature

```ts
// BAD
async function saveDoc(doc: any) { ... }

// GOOD
async function saveDoc(doc: DocumentCreateRequest) { ... }
```

### Mistake 2: HTTP call in component

```vue
<!-- BAD -->
<script setup lang="ts">
import axios from 'axios'
async function load() {
  const res = await axios.get('/api/...')
  items.value = res.data
}
</script>

<!-- GOOD -->
<script setup lang="ts">
const store = useDocsStore()
onMounted(() => store.load())
</script>
```

### Mistake 3: persisting the entire state

```ts
// BAD — persists tokens AND caches; can leak PII on shared machines
defineStore('auth', () => { /* ... */ }, { persist: true })

// GOOD — explicit paths
defineStore('auth', () => { /* ... */ }, {
  persist: { key: 'aikbs.auth', paths: ['token', 'user', 'tokenExpiresAt'] },
})
```

### Mistake 4: hardcoded brand color

```vue
<!-- BAD -->
<style scoped>
.button { background: #1677ff; color: white; }
</style>

<!-- GOOD -->
<style scoped>
.button {
  background: var(--kb-color-primary);
  color: var(--kb-color-on-primary);
}
</style>
```

The white-label scrub (T1.23) replaces `tokens.css`; every component
follows. Hardcoded colors force a per-file rewrite.

### Mistake 5: click handler without keyboard activation

```vue
<!-- BAD -->
<div class="doc-card" @click="open">...</div>

<!-- GOOD — semantic + keyboard -->
<button type="button" class="doc-card" @click="open">...</button>
```

### Mistake 6: bundle bloat — heavy library at top of main.ts

```ts
// BAD — ships monaco to every route
// src/main.ts
import 'monaco-editor'   // 5MB
```

```ts
// GOOD — lazy-load
// src/views/agent/Editor.vue
const MonacoEditor = defineAsyncComponent(() => import('monaco-editor'))
```

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3 AC-F1–F5, §3.4 AC bundle
- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 frontend box, §4.6 persistence
- `.trellis/spec/frontend/component-guidelines.md`
- `.trellis/spec/frontend/state-management.md`
- `.trellis/spec/frontend/type-safety.md`
- `.trellis/spec/frontend/hook-guidelines.md`
- `frontend/.eslintrc.cjs` — dormant config, activated in T1.22
- `Makefile` `lint-frontend`, `scrub-audit`