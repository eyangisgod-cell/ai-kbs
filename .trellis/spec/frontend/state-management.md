# State Management

> How state is organized in `frontend/` (Pinia 2 + Vue 3 Composition API).

---

## Overview

State is split into **four categories** by ownership and lifetime:

| Category | Owner | Lifetime | Examples |
|----------|-------|----------|----------|
| **Local component state** | The component itself (`ref`, `reactive`) | While the component is mounted | form draft, modal open flag, hover state |
| **Global client state** | A Pinia store (`stores/<domain>.ts`) | Across the whole session | auth (user, token), UI (toasts, sidebar collapse), workspace binding |
| **Server state** | A Pinia store backed by `api/<resource>.ts` | Until explicitly invalidated | knowledge-base list, chat history, agent configs |
| **URL state** | The router (path, query, params) | Survives page reload via URL | current document `docCode`, search query, pagination |

Three principles:

1. **Default to local.** A `ref` in `<script setup>` is the right answer
   for state only one component reads and writes. Promote to global
   only when a second consumer appears.
2. **Server state lives in a store**, not a component. Components read
   `store.documents`; the store calls `api.list()`; the component never
   sees the fetch.
3. **URL state for shareable / deep-linkable data.** A document detail
   page gets its `docCode` from the route param, not from a store. A
   search query goes in the query string so the URL is shareable.

The framework is **Pinia 2** in composition-API style (`defineStore(name, () => { ... })`).
Persistence is via **`pinia-plugin-persistedstate`** with selective
`paths` — only `auth.token` and `auth.user` survive a hard reload;
domain caches do not.

---

## State Categories (Decision Tree)

```
Is it derivable from props + URL?
├── yes  → computed (don't store at all)
└── no
    ├── Is it needed by ≥ 2 components or a route guard?
    │   ├── no   → local ref in <script setup>
    │   └── yes
    │       ├── Is it a value the user entered / UI toggled?
    │       │   ├── yes → Pinia store (auth, ui, workspace)
    │       │   └── no  → it is server state — Pinia store + api/*
    │       └── Does it need to survive reload?
    │           ├── yes → persisted via pinia-plugin-persistedstate (token, user)
    │           └── no  → in-memory only (current knowledge cache, list pages)
    └── Does it represent the user's location in the app?
        └── yes → URL (path / query / param)
```

---

## When to Use Global State

Promote state from a component to a Pinia store when **any** of the
following is true:

- A second component needs to read it (sibling, ancestor, or unrelated route).
- A route guard needs to read it (auth check, permission check).
- The state outlives the component (a closed modal should remember
  something for when it reopens).
- A non-trivial action mutates it (cross-cutting `save` / `delete` /
  `refresh` that needs to coordinate with a backend call).

If none of those apply, the state stays local.

### Anti-promotion

Do NOT promote:

- "I might need it later" → wait until you do.
- "It's easier to test as a store" → that is a testability smell;
  extract a composable instead.
- "It feels global" → if you cannot name a second consumer, it is local.

---

## Server State (Pinia + API)

Server data lives in a **domain store** (`stores/<domain>.ts`) that
calls the matching `api/<resource>.ts`. The store exposes:

- **State**: `data`, `isLoading`, `error` (typed, not `any`).
- **Actions**: `list()`, `get(id)`, `create(payload)`, `update(id, payload)`,
  `delete(id)`. Actions call `api/*` and update state on success.
- **No HTTP**: stores never see `axios` / `fetch` / `URLSearchParams`.

```ts
// stores/knowledge-base.ts
export const useKnowledgeBaseStore = defineStore('knowledge-base', () => {
  const items = ref<KnowledgeBase[]>([])
  const isLoading = ref(false)
  const error = ref<ErrorEnvelope | null>(null)

  async function list() {
    isLoading.value = true
    error.value = null
    try {
      items.value = await api.knowledgeBase.list()
    } catch (e) {
      error.value = toErrorEnvelope(e)
    } finally {
      isLoading.value = false
    }
  }

  return { items, isLoading, error, list }
})
```

Components:

```vue
<script setup lang="ts">
const store = useKnowledgeBaseStore()
onMounted(() => store.list())
</script>

<template>
  <div v-if="store.isLoading">Loading...</div>
  <div v-else-if="store.error" role="alert">
    {{ store.error.message }}
  </div>
  <ul v-else>
    <KnowledgeBaseListItem
      v-for="kb in store.items"
      :key="kb.id"
      :kb="kb"
    />
  </ul>
</template>
```

### Refresh strategy

| Pattern | When |
|---------|------|
| `onMounted` → `store.list()` | Page-load only; static-ish lists |
| Store action `refresh()` called by router guard | Refresh on route entry |
| Polling / SSE / WebSocket | Live data (chat, agent run status) |
| Optimistic update on action | Local-first (typing in a search box) |

For M1, the default is `onMounted` + manual `refresh()` from the
component. Real-time channels land in M2 (chat) and M3 (GA
verification).

### Caching

M1 does NOT cache server state across pages by default. Each page
mount triggers a fresh fetch; caches (TanStack Query, Pinia Colada)
land in M2 if real workloads show the round-trip is wasteful.

---

## URL State

The router is the source of truth for navigation. A page reads:

| Need | Source | Example |
|------|--------|---------|
| The resource being viewed | route param | `/knowledge/:docCode` → `useRoute().params.docCode` |
| A search query | query string | `?q=hello` → `useRoute().query.q` |
| Pagination | query string | `?page=2&pageSize=20` |
| Filters / sort | query string | `?status=active&sort=-updated_at` |
| Modal vs inline | query string or hash | `?modal=true`, `#settings` |

### Updating URL state

- **Read-only** URL state (resource id, deep link): the router sets it.
- **Writable** URL state (search, filters): use `router.replace` (not
  `push`) so back-button doesn't accumulate one entry per keystroke.
- **Debounce** text inputs that go to the query string (300 ms).

```ts
// composables/useSearchQuery.ts
export function useSearchQuery() {
  const route = useRoute()
  const router = useRouter()
  const query = ref(route.query.q?.toString() ?? '')
  let timer: ReturnType<typeof setTimeout> | null = null

  watch(query, (v) => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => {
      router.replace({ query: { ...route.query, q: v || undefined } })
    }, 300)
  })

  return query
}
```

---

## Persistence

| Field | Persisted? | Why |
|-------|------------|-----|
| `auth.token` | Yes (localStorage) | Stay logged in across reload |
| `auth.user` | Yes (localStorage) | Render avatar immediately on reload |
| `auth.refreshToken` | No (sessionStorage) | Limits blast radius of XSS |
| `ui.sidebarCollapsed` | Yes (localStorage) | User preference |
| `ui.theme` | Yes (localStorage) | User preference |
| `ui.activeWorkspaceId` | Yes (localStorage) | Resume last workspace |
| `knowledgeBase.items` | No | Server state; refetch on mount |
| `chat.history` | No | Server state; restore from backend per session |
| Form drafts (per-page) | No | Don't risk stale drafts; user can re-enter |

### Mechanism

`pinia-plugin-persistedstate` with explicit `paths` per store:

```ts
// stores/auth.ts
export const useAuthStore = defineStore('auth', () => { /* ... */ }, {
  persist: {
    key: 'aikbs.auth',
    storage: localStorage,
    paths: ['token', 'user', 'refreshExpiresAt'],
  },
})
```

NEVER persist `*` (the entire store). Pick fields explicitly.

### Security caveat

localStorage is XSS-readable. AI-KBS sanitizes all rendered HTML
(via DOMPurify in `utils/markdownDomPurify.ts`) and does not store
long-lived bearer tokens in `localStorage` without an expiry. For
high-value sessions, prefer the backend's secure cookie path
(decision deferred to an ADR that lands before M3 GA — for M1 the
JWT-in-localStorage + short-lived refresh-token pattern is the
baseline).

---

## Derived State

Use `computed` for everything derivable from props or store state. The
rules:

- A computed MUST be pure. No side effects (no fetch, no mutation of
  other refs).
- A computed MAY return a new array/object — Vue caches by reference
  identity, so downstream consumers re-render only when the returned
  reference changes.
- If you need a side effect when derived state changes, use
  `watchEffect` (or `watch` with an explicit source), not a computed.

```ts
// GOOD — pure derived state
const visibleItems = computed(() =>
  store.items.filter(i => i.status === 'active'))

// BAD — derived with side effect
const visibleItems = computed(() => {
  const items = store.items.filter(i => i.status === 'active')
  telemetry.emit('filter_applied', { count: items.length })   // side effect
  return items
})
```

---

## Common Mistakes

### Mistake 1: store for one component

```ts
// BAD — single-component state in a store
// stores/modal-open.ts
export const useModalOpenStore = defineStore('modal-open', () => {
  const open = ref(false)
  return { open }
})
```

```vue
<!-- GOOD — local ref -->
<script setup lang="ts">
const open = ref(false)
</script>
```

### Mistake 2: HTTP in a store action with no error handling

```ts
// BAD — silently fails, leaves isLoading=true forever
async function load() {
  isLoading.value = true
  items.value = await api.knowledgeBase.list()
  isLoading.value = false   // ← skipped on error
}
```

```ts
// GOOD — try/catch/finally
async function load() {
  isLoading.value = true
  error.value = null
  try {
    items.value = await api.knowledgeBase.list()
  } catch (e) {
    error.value = toErrorEnvelope(e)
  } finally {
    isLoading.value = false
  }
}
```

### Mistake 3: duplicating server state in URL

```vue
<!-- BAD — store docCode in BOTH the store and the URL; they drift -->
<script setup lang="ts">
const store = useKnowledgeStore()
store.currentDocCode = route.params.docCode
// then later: store.currentDocCode = 'abc'  ← bypasses URL
</script>
```

```vue
<!-- GOOD — single source of truth = route param -->
<script setup lang="ts">
const store = useKnowledgeStore()
const route = useRoute()
watchEffect(() => store.setCurrentDocCode(route.params.docCode as DocCode))
</script>
```

### Mistake 4: persisting tokens / secrets in localStorage without expiry

```ts
// BAD — token persists forever, even after server-side revoke
persist: { paths: ['token'] }
```

```ts
// GOOD — token + expiry; refresh-on-mount verifies still-valid
persist: {
  paths: ['token', 'tokenExpiresAt'],
  beforeHydrate: (ctx) => {
    if (ctx.store.tokenExpiresAt && Date.now() > ctx.store.tokenExpiresAt) {
      ctx.store.token = null
      ctx.store.user = null
    }
  },
}
```

### Mistake 5: shallowRef for nested objects

```ts
// BAD — mutations don't trigger reactivity
const doc = shallowRef<Document>({ /* ... */ })
doc.value.title = 'new'   // ← no reactive trigger
```

```ts
// GOOD — ref for nested objects you mutate; shallowRef for read-only large lists
const doc = ref<Document>({ /* ... */ })
const list = shallowRef<readonly Document[]>([])   // immutable list
```

### Mistake 6: store cross-importing other stores for every read

```ts
// BAD — auth store reaches into knowledge store
const auth = useAuthStore()
const knowledge = useKnowledgeStore()
auth.lastViewedDoc = knowledge.current?.docCode   // coupling both directions
```

```ts
// GOOD — one-way dependency; reads happen in the component
const auth = useAuthStore()
const knowledge = useKnowledgeStore()
// Component reads both, decides what to do.
```

---

## Examples

### Good: a chat store with optimistic updates

```ts
// stores/chat.ts
export const useChatStore = defineStore('chat', () => {
  const messages = ref<ChatMessage[]>([])
  const isStreaming = ref(false)
  const error = ref<ErrorEnvelope | null>(null)

  async function send(content: string) {
    error.value = null
    const temp: ChatMessage = { id: `tmp_${Date.now()}`, content, role: 'user', status: 'pending' }
    messages.value.push(temp)
    try {
      const final = await api.chat.send(temp.id, content)
      const i = messages.value.findIndex(m => m.id === temp.id)
      if (i >= 0) messages.value[i] = final
    } catch (e) {
      error.value = toErrorEnvelope(e)
      messages.value = messages.value.filter(m => m.id !== temp.id)
    }
  }

  return { messages, isStreaming, error, send }
})
```

### Good: a UI store

```ts
// stores/ui.ts
export const useUiStore = defineStore('ui', () => {
  const sidebarCollapsed = ref(false)
  const theme = ref<'light' | 'dark'>('light')
  const toasts = ref<Toast[]>([])

  function toggleSidebar() { sidebarCollapsed.value = !sidebarCollapsed.value }
  function setTheme(t: 'light' | 'dark') { theme.value = t }
  function pushToast(t: Omit<Toast, 'id'>) {
    const id = `t_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
    toasts.value.push({ id, ...t })
    setTimeout(() => toasts.value = toasts.value.filter(x => x.id !== id), 4000)
  }

  return { sidebarCollapsed, theme, toasts, toggleSidebar, setTheme, pushToast }
}, {
  persist: {
    key: 'aikbs.ui',
    paths: ['sidebarCollapsed', 'theme'],
  },
})
```

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 frontend box, §4.6 state persistence
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3 AC-F5 (auth store + router guards)
- `.trellis/spec/frontend/directory-structure.md` — `src/stores/` placement
- `.trellis/spec/frontend/component-guidelines.md` — how components consume stores
- `.trellis/spec/frontend/type-safety.md` — typed store state