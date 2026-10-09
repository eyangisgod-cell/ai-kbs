# Frontend Directory Structure

> How Vue 3 code is organized in `frontend/` (Vue 3 + Vite 5 + Pinia 2 + Vue Router 4 + TypeScript).

---

## Overview

The frontend is a **single-page application** built with Vue 3 and
TypeScript. Package manager is **pnpm** with a workspace file at
`frontend/pnpm-workspace.yaml`. The build is Vite 5; the host bundle is
under `frontend/src/` and the embed bundle is at `frontend/embed.html`
+ `frontend/src/embed-main.ts` (separate entry for the chat-embed
widget).

Three principles govern the layout:

1. **Layered responsibility** — `src/api` calls the backend; `src/stores`
   hold state; `src/views` are routed pages; `src/components` are reusable
   presentation. A component in `src/components/` MUST NOT call `src/api/`
   directly; it must read from a store.
2. **Feature folders inside `views/` and `components/`** — when a folder
   represents a feature (knowledge-base, agent, chat), the file name
   inside the folder is the resource, not the type. `views/knowledge-base/List.vue`,
   not `views/knowledge-base/KnowledgeBaseList.vue`.
3. **No business logic in `main.ts` or router config** — wiring only.
   Composables own cross-component behavior; stores own cross-route state.

---

## Directory Layout

```
frontend/
├── package.json                          # vue 3, vite 5, pinia 2, vue-router 4, axios, typescript
├── pnpm-lock.yaml
├── pnpm-workspace.yaml                   # monorepo hints (current state: single app)
├── tsconfig.json                         # strict mode (see type-safety.md)
├── tsconfig.node.json                    # for vite.config.ts and node-side scripts
├── env.d.ts                              # vite client types
├── vite.config.ts                        # build config (entry aliases, chunk splits)
├── Dockerfile                            # multi-stage; serves dist/ via nginx
├── .dockerignore
├── index.html                            # SPA host entry
├── embed.html                            # separate entry for the embed widget
│
├── public/                               # served verbatim at root
│   ├── favicon.svg
│   └── locales/                          # i18n message catalogs (en.json, zh-CN.json)
│
├── src/
│   ├── main.ts                           # SPA bootstrap (mount, router, i18n, pinia)
│   ├── App.vue                           # root component (router-view + layout)
│   ├── env.d.ts                          # ImportMetaEnv augmentation
│   │
│   ├── api/                              # HTTP boundary (axios + JWT injection)
│   │   ├── http.ts                       # axios instance; JWT interceptor; envelope unwrap
│   │   ├── auth.ts                       # login / logout / refresh
│   │   ├── knowledge-base.ts
│   │   ├── knowledge.ts
│   │   ├── chat.ts
│   │   ├── agent.ts
│   │   ├── wiki.ts
│   │   ├── datasource.ts
│   │   ├── embed.ts                      # embed-mode specific (chunked embeddings)
│   │   ├── tenant.ts
│   │   ├── model.ts                      # LLM provider config
│   │   ├── chunker.ts
│   │   ├── organization.ts
│   │   ├── skill.ts
│   │   ├── system/                       # system endpoints (health, version)
│   │   └── initialization/               # first-run setup
│   │
│   ├── stores/                           # Pinia stores (composition API style)
│   │   ├── auth.ts                       # user + token; persisted via localStorage
│   │   ├── knowledge-base.ts
│   │   ├── knowledge.ts
│   │   ├── chat.ts
│   │   ├── agent.ts
│   │   ├── ui.ts                         # toasts, modal state, sidebar collapse
│   │   └── workspace.ts                  # active workspace + sync-state binding
│   │
│   ├── router/
│   │   ├── index.ts                      # createRouter; lazy imports
│   │   ├── guards.ts                     # auth/permission guards
│   │   └── routes.ts                     # typed route table
│   │
│   ├── views/                            # routed pages (one folder per feature)
│   │   ├── auth/
│   │   ├── knowledge-base/
│   │   │   ├── List.vue
│   │   │   ├── Detail.vue
│   │   │   └── Create.vue
│   │   ├── knowledge/
│   │   ├── chat/
│   │   ├── agent/
│   │   ├── dashboard/
│   │   ├── settings/
│   │   └── error/
│   │       ├── NotFound.vue              # 404
│   │       └── Forbidden.vue             # 403
│   │
│   ├── components/                       # reusable presentation
│   │   ├── common/                       # buttons, inputs, dialogs (no business knowledge)
│   │   ├── chat/
│   │   ├── credentials/                  # LLM provider API key forms
│   │   ├── settings/
│   │   ├── knowledge-base/
│   │   ├── GlobalCommandPalette/         # cmd+k style global action palette
│   │   └── icons/                         # icon components (auto-imported)
│   │
│   ├── composables/                     # reusable stateful logic (Vue 3 use*)
│   │   ├── useAuth.ts                    # thin store glue for components
│   │   ├── usePagination.ts
│   │   ├── useDebounce.ts
│   │   ├── useToast.ts
│   │   ├── useClipboard.ts
│   │   ├── useMarkdown.ts                # markdown render + sanitize
│   │   └── useKnowledgeChunks.ts
│   │
│   ├── hooks/                            # legacy (Vue 2 mixin-like); kept for fork parity
│   │                                   # new code goes in composables/
│   │
│   ├── directives/                       # Vue directives (v-permission, v-tooltip)
│   │   └── permission.ts                 # reads user role from auth store
│   │
│   ├── utils/                            # framework-agnostic helpers
│   │   ├── markdownDomPurify.ts
│   │   ├── referenceSources.ts
│   │   ├── knowledgeChunksDisplay.test.mjs
│   │   ├── zoom.ts
│   │   └── api-base.ts                   # shared by api/ + stores/
│   │
│   ├── types/                            # shared TypeScript types (no runtime)
│   │   ├── envelope.ts                   # mirrors backend envelope v1.0
│   │   ├── doc.ts
│   │   ├── binding.ts
│   │   ├── user.ts
│   │   └── branded.ts                    # branded ID types (DocCode, Version, UserID)
│   │
│   ├── styles/                           # global CSS + tokens
│   │   ├── tokens.css                    # CSS custom properties (theme)
│   │   ├── reset.css
│   │   └── index.css
│   │
│   ├── assets/                           # processed at build time (images, fonts)
│   │   ├── images/
│   │   └── fonts/
│   │
│   ├── i18n/                             # message catalogs + helpers
│   │   ├── index.ts
│   │   ├── en.ts
│   │   └── zh-CN.ts
│   │
│   ├── wailsjs/                          # generated bindings (do not edit by hand)
│   │   ├── runtime/
│   │   └── go/main/
│   │
│   ├── embed-main.ts                     # entry for embed.html (separate bundle)
│   ├── embed.html                        # served at /embed.html
│   └── embedContext.ts                   # state for embed mode
│
├── tests/                                # vitest specs (T1.22 lands; current state in .vue.test.mjs)
│   ├── unit/
│   ├── component/
│   └── e2e/                              # Playwright (planned for T1.22 follow-up)
│
└── dist/                                 # build output (gitignored)
```

---

## Module Organization

### `src/api/` — HTTP boundary only

- One file per resource; method names mirror the backend endpoints.
- Functions return a **typed payload**, not an envelope. The axios
  interceptor in `src/api/http.ts` unwraps `envelope.data` and
  rejects with a normalized error on `envelope.success === false`.
- Errors carry the original `request_id` for support tickets; see
  `types/envelope.ts`.
- No component imports `axios` directly. Components import from
  `src/api/<area>.ts` and call `knowledgeBase.list()`.

### `src/stores/` — Pinia (composition style)

- One file per domain; `defineStore(name, () => { ... })`.
- Stores call `api/<area>.ts`, never `fetch` / `axios` directly.
- Persistent fields (token, user) use `localStorage` via a Pinia
  plugin (`pinia-plugin-persistedstate`). Don't roll your own.

### `src/views/` vs `src/components/`

| Use | Folder |
|-----|--------|
| A page mounted at a route | `src/views/<feature>/<Name>.vue` |
| A widget embedded in a view or other component | `src/components/<feature>/<Name>.vue` |
| A reusable control with no business meaning | `src/components/common/<Name>.vue` |

The rule: **a view is the destination of a navigation; a component is
not.** A `KnowledgeListItem` used inside `KnowledgeList` is a component.
A page that shows knowledge + chat side-by-side is a view, even if it
embeds `KnowledgeList`.

### `src/composables/` — Vue 3 hooks

- `use*` prefix.
- Return reactive refs and functions; never mutate global state
  implicitly.
- Async composables return `{ data, isLoading, refetch }` shape; never
  raw promises.

### `src/router/` — routing

- `routes.ts` exports the typed route table; `index.ts` wires it to
  `createRouter`.
- `guards.ts` reads auth + permission stores; never reads `localStorage`
  directly.

### `src/directives/` — Vue directives

- One file per directive (`permission.ts`, `tooltip.ts`).
- Directives are for DOM-level concerns (visibility, focus, tooltip
  positioning). Business gating goes through `useAuth` / router guards.

### `src/utils/` — pure helpers

- Framework-agnostic. If a util imports from `vue` or `pinia`, it
  belongs in `composables/` instead.
- Pair every util with a `.test.mjs` next to it (see
  `src/utils/knowledgeChunksDisplay.test.mjs` as the template).

### `src/types/` — shared types

- Types only, no runtime code.
- Mirrors backend contracts (e.g., `types/envelope.ts` mirrors the
  backend's envelope v1.0).
- Branded types (`DocCode`, `Version`, `UserID`) live here; see
  `type-safety.md`.

---

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Component file | PascalCase.vue | `KnowledgeList.vue` |
| Composable | `use<Name>.ts` | `usePagination.ts` |
| Store | lowercase.ts by domain | `auth.ts`, `knowledge-base.ts` |
| API module | lowercase.ts by resource | `knowledge-base.ts`, `chat.ts` |
| View | PascalCase.vue inside `views/<feature>/` | `views/knowledge-base/List.vue` |
| Type | PascalCase, suffix `-T` only when it disambiguates | `Document`, `LoginResponse` |
| Branded ID | PascalCase + `Id` | `DocCode`, `UserId`, `BindingId` |
| Test | `<filename>.test.mjs` or `.spec.ts` | `knowledgeChunksDisplay.test.mjs` |
| CSS class | `kebab-case`, prefixed by component | `.kb-knowledge-list__item` |
| i18n key | `dot.separated` | `kb.document.title` |

### Folder vs file

- A folder exists when ≥ 3 related files share a prefix.
- Otherwise flat file (e.g., `src/components/SearchBar.vue`,
  not `src/components/search/SearchBar.vue`).

---

## Cross-Module Import Rules (must not cycle)

```
main.ts  →  router/index.ts  →  views/<feature>/<Page>.vue
                              ↘  components/<feature>/<Widget>.vue
                                                       ↘  composables/use<X>.ts
                                                       ↘  stores/<domain>.ts
                                                                          ↘  api/<resource>.ts
                                                                                          ↘  utils/<helper>.ts

src/types/* is a pure leaf (no imports from src/* except env.d.ts)
src/utils/* is a pure leaf (no imports from src/* except env.d.ts)
src/api/* imports from src/types/*, src/utils/* only
src/composables/* imports from src/types/*, src/utils/*, src/stores/* (read-only)
src/stores/* imports from src/api/*, src/types/*, src/utils/* (no UI)
src/components/* imports from src/composables/*, src/stores/* (read-only), src/types/*
src/views/* imports from src/components/*, src/composables/*, src/stores/*, src/api/* (page-level only)
```

No `src/views/` may import `src/api/` AND `src/stores/` for the same
call — pick the store, have the store call the API.

---

## Where to Put New Code

| You're adding... | Put it in |
|------------------|-----------|
| A new HTTP call to the backend | `src/api/<resource>.ts` |
| A new Pinia store | `src/stores/<domain>.ts` |
| A new routed page | `src/views/<feature>/<Name>.vue` + register in `router/routes.ts` |
| A reusable widget | `src/components/<feature>/<Name>.vue` |
| A reusable stateful hook | `src/composables/use<X>.ts` |
| A pure helper | `src/utils/<name>.ts` (+ `.test.mjs`) |
| A new cross-cutting type | `src/types/<name>.ts` |
| A new CSS token / global style | `src/styles/tokens.css` (or extend existing) |
| A new i18n key | extend `src/i18n/en.ts` and `src/i18n/zh-CN.ts` in the same PR |
| A new icon | `src/components/icons/<Name>.svg` + auto-import registration |

---

## Examples

- A well-organized feature: `views/knowledge-base/` with `List.vue`,
  `Detail.vue`, `Create.vue`; companion `stores/knowledge-base.ts`,
  `api/knowledge-base.ts`, `components/knowledge-base/` with reusable
  list/filter widgets.
- A well-organized composable: `composables/useDebounce.ts` exporting
  `useDebounce<T>(value: Ref<T>, ms: number): Ref<T>` with a paired
  `useDebounce.test.mjs`.
- A well-organized store: `stores/auth.ts` exposes `user`, `isAuthed`,
  `login`, `logout`; persistence via `pinia-plugin-persistedstate`;
  reads `request_id` from failed logins into an error toast.

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 frontend box, §4.6 state persistence
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3 AC-F1–F5
- `.trellis/spec/frontend/component-guidelines.md`
- `.trellis/spec/frontend/state-management.md`
- `.trellis/spec/frontend/type-safety.md`
- `.trellis/spec/frontend/hook-guidelines.md`
- `frontend/pnpm-workspace.yaml`, `frontend/package.json`