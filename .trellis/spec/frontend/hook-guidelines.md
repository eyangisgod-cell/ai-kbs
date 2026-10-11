# Hook Guidelines

> How Vue 3 composables (a.k.a. "hooks") are written and shared.

---

## Overview

In AI-KBS, "hook" means **Vue 3 composable** — a function that uses
Vue's reactivity primitives (`ref`, `computed`, `watch`, lifecycle
hooks) and may return reactive values + functions. They live in
`src/composables/` and are the **only** way to share stateful logic
between multiple components.

`src/hooks/` exists in the fork (Vue 2 mixin-style code) but **new
code goes in `composables/`**. Vue 2 mixins are not allowed in new code.

Three principles:

1. **Pure of side effects at module load.** A composable MAY set up
   reactivity, but must not `fetch` / `console.log` / mutate a global
   on import. Behavior runs only when the consumer calls the composable.
2. **Return reactive values, not raw promises.** An async composable
   returns `{ data, isLoading, error, refetch }`, not the promise itself.
3. **No global mutation.** A composable may READ from stores (it's
   their job to bridge stores to components), but it MUST NOT mutate
   other stores or write to `localStorage` directly.

---

## Custom Composable Patterns

### File shape

```
src/composables/
├── useDebounce.ts
├── usePagination.ts
├── useAuth.ts          # thin store glue
├── useToast.ts
├── useClipboard.ts
├── useMarkdown.ts
└── useKnowledgeChunks.ts
```

Every composable file:

- Default-exports OR named-exports a single function with `use*` prefix.
- Has a sibling `*.test.mjs` (or `*.spec.ts`) next to it.
- Lives in `src/composables/` (NOT `src/hooks/`).

### Function shape

```ts
// src/composables/useDebounce.ts
import { ref, watch, type Ref } from 'vue'

export function useDebounce<T>(source: Ref<T>, ms = 100): Ref<T> {
  const debounced = ref(source.value) as Ref<T>
  let timer: ReturnType<typeof setTimeout> | null = null

  watch(source, (v) => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => { debounced.value = v }, ms)
  })

  onUnmounted(() => { if (timer) clearTimeout(timer) })

  return debounced
}
```

Rules:

- The first letter of the return type is **capitalized** (it's a value/type).
- Param types are explicit on the signature (public surface).
- Return type is explicit when the function is non-trivial; inferred
  when it's a simple ref / computed chain.

---

## Naming Conventions

| Pattern | Convention | Example |
|---------|------------|---------|
| File | `use<Name>.ts` (PascalCase after `use`) | `usePagination.ts`, `useMarkdown.ts` |
| Function | `use<Name>` | `useDebounce`, `useAuth` |
| Ref return | lowerCamelCase noun | `data`, `isLoading`, `error`, `value` |
| Function return | lowerCamelCase verb | `submit`, `refetch`, `cancel`, `reset` |
| Internal helper | lowerCamelCase, not exported | `debounced`, `timer` |
| Naming convention | Don't prefix with `composable` or `hook` | — |

### What `use*` returns

| Composable shape | When |
|------------------|------|
| `Ref<T>` | A single reactive value (`useDebounce`, `useClipboard`) |
| Multiple values in an object | An async / multi-state composable (`usePagination`, `useToast`) |
| `undefined` (side-effect only) | `useMountOnce(fn)`-style; rare |
| Pair of `[Ref<T>, (v: T) => void]` | a controlled prop helper (`useVModel`) |

---

## Data Fetching

Data fetching is **always** done through a Pinia store, which calls
`api/<resource>.ts`. A composable MAY wrap a store action for ergonomic
consumption but MUST NOT call `api/*` directly.

### Why a composable on top of a store?

A store owns **state**; a composable owns **behavior the component
needs now**. Example: pagination.

```ts
// stores/docs.ts
export const useDocsStore = defineStore('docs', () => {
  const items = ref<Document[]>([])
  const total = ref(0)
  async function fetchPage(page: number, pageSize: number) {
    const res = await api.docs.list({ page, pageSize })
    items.value = res.items
    total.value = res.total
  }
  return { items, total, fetchPage }
})
```

```ts
// composables/usePagination.ts
export function usePagination(opts: {
  fetcher: (page: number, pageSize: number) => Promise<{ items: unknown[]; total: number }>
  pageSize?: number
}) {
  const page = ref(1)
  const items = ref<unknown[]>([])
  const total = ref(0)
  const isLoading = ref(false)
  const error = ref<ErrorEnvelope | null>(null)

  async function load() {
    isLoading.value = true
    error.value = null
    try {
      const res = await opts.fetcher(page.value, opts.pageSize ?? 20)
      items.value = res.items
      total.value = res.total
    } catch (e) {
      error.value = toErrorEnvelope(e)
    } finally {
      isLoading.value = false
    }
  }

  watch(page, load, { immediate: true })

  return { page, items, total, isLoading, error, refetch: load, setPage: (v: number) => { page.value = v } }
}
```

```vue
<!-- views/docs/List.vue -->
<script setup lang="ts">
import { usePagination } from '@/composables/usePagination'
import { useDocsStore } from '@/stores/docs'

const store = useDocsStore()
const { page, items, total, isLoading, setPage } = usePagination({
  fetcher: (p, ps) => store.fetchPage(p, ps),
})
</script>
```

The composable is generic over the fetcher; the store provides the
typed fetcher. The component composes both.

### Async composable contract

An async composable MUST return `{ data, isLoading, error, refetch }`
(or equivalent typed names). It MUST NOT return a raw promise.

| Field | Type | Meaning |
|-------|------|---------|
| `data` | `Ref<T>` | The fetched value; `null` or initial until first load |
| `isLoading` | `Ref<boolean>` | True while a fetch is in flight |
| `error` | `Ref<ErrorEnvelope \| null>` | Last error, if any |
| `refetch` | `() => Promise<void>` | Re-run the operation |

```ts
// GOOD
return { data, isLoading, error, refetch }

// BAD — raw promise
return fetchPromise(...)
</template>
```

---

## Lifecycle in Composables

A composable that opens subscriptions (timers, intervals, observers,
WebSockets) MUST clean up.

| Resource | Cleanup |
|----------|---------|
| `setTimeout` / `setInterval` | `clearTimeout` / `clearInterval` in `onUnmounted` |
| `addEventListener` | `removeEventListener` in `onUnmounted` |
| `IntersectionObserver` / `ResizeObserver` | `observer.disconnect()` |
| WebSocket / EventSource | `.close()` in `onUnmounted` |
| `watch` with side effect | track via `onUnmounted(() => watcher())` (the watcher's stop handle) |

If the composable is consumed in `<script setup>`, Vue's component
lifecycle handles the cleanup automatically. If the composable is
consumed in another composable or store, pass the lifecycle host:

```ts
// src/composables/useMount.ts
export function useMount(fn: () => void | (() => void)) {
  onMounted(() => {
    const cleanup = fn()
    if (typeof cleanup === 'function') onUnmounted(cleanup)
  })
}
```

---

## Sharing Stateful Logic

When the same `ref` + watcher + cleanup block appears in ≥ 3 components,
extract it to `src/composables/`. Two is borderline; one is local.

### Extraction ladder

| Same pattern in | Action |
|-----------------|--------|
| 1 component | Inline `<script setup>` |
| 2 components | Local composable in `src/composables/use<X>.ts` |
| ≥ 3 components | Promote to `src/composables/use<X>.ts` + add a `.test.mjs` |

If the extracted composable needs store-level state (survives
component unmount), promote it further to a store. The line:

- **Composable** = per-instance, scoped to the component tree it runs in.
- **Store** = app-wide, survives component unmount, single instance per Pinia app.

---

## Common Mistakes

### Mistake 1: composable returns raw promise

```ts
// BAD
export function useUser() {
  return api.user.get()   // caller awaits it; no reactivity, no error state
}
```

```ts
// GOOD
export function useUser() {
  const data = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<ErrorEnvelope | null>(null)
  async function load() {
    isLoading.value = true
    try { data.value = await api.user.get() }
    catch (e) { error.value = toErrorEnvelope(e) }
    finally { isLoading.value = false }
  }
  load()
  return { data, isLoading, error, refetch: load }
}
```

### Mistake 2: composable mutates another store

```ts
// BAD — composable with global side effect
export function useFormSubmit(payload: FormPayload) {
  const auth = useAuthStore()             // ← mutation in composable
  auth.lastSubmittedAt = Date.now()
  return api.form.submit(payload)
}
```

```ts
// GOOD — composable returns; component (or store) decides what to persist
export function useFormSubmit() {
  return { submit: (p: FormPayload) => api.form.submit(p) }
}
```

### Mistake 3: composable forgets cleanup

```ts
// BAD — listener leaks across re-mounts
export function useWindowWidth() {
  const width = ref(window.innerWidth)
  window.addEventListener('resize', () => { width.value = window.innerWidth })
  return width
}
```

```ts
// GOOD — cleanup registered
export function useWindowWidth() {
  const width = ref(window.innerWidth)
  function onResize() { width.value = window.innerWidth }
  onMounted(() => window.addEventListener('resize', onResize))
  onUnmounted(() => window.removeEventListener('resize', onResize))
  return width
}
```

### Mistake 4: composable inside another without explicit lifecycle

```ts
// BAD — composable chained outside a component lifecycle
export function useAuthedFetcher() {
  const auth = useAuthStore()    // ← if called outside setup() Vue warns
  return async (url: string) => api.get(url, { token: auth.token })
}
```

```ts
// GOOD — composables are called inside setup() or another composable's body
// The store itself is set up by Pinia; the auth composable just reads.
```

### Mistake 5: implicit `any` from missing return type

```ts
// BAD — return type is the union that defeats inference
export function usePagination(opts: any) {
  // ...
  return { page, items, isLoading }   // ← caller sees `Ref<unknown>` for items
}
```

```ts
// GOOD — generic with inferable return
export function usePagination<T>(opts: UsePaginationOptions<T>) {
  const items = ref<T[]>([]) as Ref<T[]>
  // ...
  return { page, items, isLoading, error, refetch, setPage }
}
```

### Mistake 6: composable does the work but doesn't expose the result

```ts
// BAD — composable does work but returns nothing
export function useDocumentLoader(docCode: Ref<DocCode>) {
  watch(docCode, async (code) => {
    await api.docs.get(code)   // ← result thrown away
  })
}
```

```ts
// GOOD — return the reactive state
export function useDocumentLoader(docCode: Ref<DocCode>) {
  const doc = ref<Document | null>(null)
  const isLoading = ref(false)
  const error = ref<ErrorEnvelope | null>(null)
  watch(docCode, async (code) => {
    isLoading.value = true
    try { doc.value = await api.docs.get(code) }
    catch (e) { error.value = toErrorEnvelope(e) }
    finally { isLoading.value = false }
  })
  return { doc, isLoading, error }
}
```

---

## Examples

### Good: a debounced search composable

```ts
// src/composables/useDebouncedSearch.ts
import { ref, watch, type Ref } from 'vue'
import type { ErrorEnvelope } from '@/types/error'

export interface UseDebouncedSearchOptions<T> {
  fetcher: (q: string) => Promise<T[]>
  delayMs?: number
}

export function useDebouncedSearch<T>(opts: UseDebouncedSearchOptions<T>) {
  const query = ref('')
  const results = ref<T[]>([]) as Ref<T[]>
  const isLoading = ref(false)
  const error = ref<ErrorEnvelope | null>(null)

  watch(query, async (q) => {
    isLoading.value = true
    error.value = null
    try {
      results.value = await opts.fetcher(q)
    } catch (e) {
      error.value = toErrorEnvelope(e)
      results.value = []
    } finally {
      isLoading.value = false
    }
  })

  return { query, results, isLoading, error }
}
```

### Good: a clipboard composable

```ts
// src/composables/useClipboard.ts
import { ref } from 'vue'

export function useClipboard() {
  const copied = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  async function copy(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text)
      copied.value = true
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => { copied.value = false }, 1500)
      return true
    } catch {
      return false
    }
  }

  return { copied, copy }
}
```

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 frontend box
- `.trellis/spec/frontend/directory-structure.md` — `src/composables/` placement
- `.trellis/spec/frontend/state-management.md` — store vs composable split
- `.trellis/spec/frontend/type-safety.md` — typed composables
- `.trellis/spec/frontend/component-guidelines.md` — how components consume composables