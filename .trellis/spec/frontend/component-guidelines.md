# Component Guidelines

> How Vue 3 components are built in `frontend/`.

---

## Overview

AI-KBS uses **Vue 3 SFC (Single File Component)** with `<script setup
lang="ts">`. Every component:

- Is one file (`<Name>.vue`).
- Uses TypeScript for props, emits, and refs.
- Reads state from a Pinia store via `useXxxStore()`, never from a
  sibling component's exposed ref.
- Emits events with `defineEmits<...>()` (typed) instead of prop callbacks.
- Owns presentation only — no HTTP calls (the store does that), no
  business rules (composables or services do that).

Three principles:

1. **Single responsibility** — a component is one widget, one dialog, one
   table row. If you can describe it with "and", split it.
2. **Unidirectional data flow** — props down, events up. No two-way
   binding (`v-model` is allowed only on form controls within a component's
   own template).
3. **A11y by default** — semantic HTML, ARIA only when needed, keyboard
   handlers paired with mouse handlers.

---

## Component Structure

Every component follows the **5-block SFC order**:

```vue
<script setup lang="ts">
// 1. imports (grouped: vue → blank, third-party → blank, project)
// 2. props / emits / slots (defineProps / defineEmits / defineSlots)
// 3. composables (useXxxStore, useDebounce, useToast, ...)
// 4. refs (reactive state local to this component)
// 5. computed + watch
// 6. lifecycle (onMounted / onUnmounted — minimize)
// 7. methods (named, exported only via defineExpose)
</script>

<template>
  <!-- 8. template: one root element unless fragment is intentional -->
</template>

<style scoped>
/* 9. scoped CSS using tokens (var(--kb-...) ); no deep selectors */
</style>
```

### Rules

- **One root element** in `<template>` unless you need a fragment (multi-root
  requires a `key` on the loop or `<template>` with `v-for`). Fragment roots
  make `aria-labelledby` and CSS target queries ambiguous.
- **No business logic in `<script setup>`** beyond local UI state. Calls
  to `api/`, store mutations that aren't user-driven, and decision
  trees go in `composables/` or `stores/`.
- **No inline `style="..."`** in the template. Use scoped CSS or tokens.
- **`defineProps` with type-only signature** (no runtime defaults unless
  the prop is genuinely optional). Required props must be obvious in the
  call site.

---

## Props Conventions

### Type-only props (preferred)

```vue
<script setup lang="ts">
interface Props {
  docCode: DocCode
  readOnly?: boolean
  version?: number
}
const props = withDefaults(defineProps<Props>(), {
  readOnly: false,
  version: 0,
})
</script>
```

`defineProps<T>()` is **compile-time only** — no runtime overhead, full
type inference at call sites, IDE autocompletion.

### Naming

| Pattern | Convention | Example |
|---------|------------|---------|
| Singular value | noun | `docCode`, `version`, `title` |
| Boolean | `is*` / `has*` / `can*` / `should*` | `isLoading`, `hasError`, `canEdit` |
| Action prop | `on*` callback | `onChange?: (v: T) => void` |
| Slot | PascalCase name | `<Header>` slot |
| Event | `verb` past tense | `update`, `submit`, `change` (matches `v-model` contract) |

### Required vs optional

- Required props go first in the interface.
- Optional props use `?:` and have a `withDefaults` entry.
- A prop that is always present in practice is NOT optional just to
  avoid a default — pick a default and document it.

### Avoid

- `props: any` — TypeScript loses the contract.
- Mutating a prop (`props.value++`) — emits `update:value` instead.
- Passing whole stores as props — components import the store directly.
- Deep optional types (`{ a?: { b?: { c?: string } } }`) — flatten or
  use discriminated unions.

---

## Emits Conventions

```vue
<script setup lang="ts">
const emit = defineEmits<{
  (e: 'update:docCode', value: DocCode): void
  (e: 'change', payload: { from: number; to: number }): void
  (e: 'submit'): void
}>()
</script>
```

### Naming

| Intent | Convention |
|--------|------------|
| Update a prop (v-model) | `update:<propName>` (`update:docCode`, `update:title`) |
| A semantic event | verb in past tense (`submit`, `change`, `select`) |
| A lifecycle-style signal | `before*` / `after*` prefix (`beforeClose`, `afterMount`) |

`v-model` integration:

```vue
<!-- Parent -->
<KnowledgeEditor v-model:docCode="doc.docCode" v-model:title="doc.title" />

<!-- Child (uses update:docCode / update:title) -->
```

### Avoid

- Emitting DOM event names (`click`, `focus`) — those bubble naturally.
- Emitting a "data" payload with no semantic — emit a specific verb + payload.
- Using `defineEmits([])` (empty array) — at least declare the empty
  type so future emit calls get checked.

---

## Slots

```vue
<script setup lang="ts">
defineSlots<{
  default(props: { item: Document; index: number }): any
  header(): any
  footer(props: { total: number }): any
}>()
</script>
```

- Named slots for repeated widgets (`#header`, `#footer`, `#actions`).
- Scoped slots when the parent needs the inner data — pass through
  `defineSlots<...>` typing.
- Don't pass JSX as a slot prop; if you need a render function, use
  a prop callback or a separate sub-component.

---

## Styling Patterns

### Tokens, not literals

Use CSS custom properties from `src/styles/tokens.css`:

```vue
<style scoped>
.btn-primary {
  background: var(--kb-color-primary);
  color: var(--kb-color-on-primary);
  border-radius: var(--kb-radius-md);
  padding: var(--kb-spacing-2) var(--kb-spacing-4);
}
</style>
```

Never hardcode `#1677ff` or `12px` in a component. The white-label
rebrand (T1.23) replaces one token file and every component follows.

### Scoped CSS (default)

`<style scoped>` — selectors are scoped via Vue's data-attribute
hashing. Avoid `:deep()` unless crossing a third-party component
boundary.

### CSS variables in style (Vue 3 supports it)

```vue
<style scoped>
.themed {
  --my-component-accent: var(--kb-color-primary);
  color: var(--my-component-accent);
}
</style>
```

This lets a parent override a child theme without `!important`.

### Composition order (large components)

For ≥ 300 lines of template or ≥ 3 logical regions, split into
sub-components in the same folder:

```
components/chat/
├── ChatWindow.vue           # composes the others
├── ChatMessageList.vue
├── ChatMessageInput.vue
└── ChatTokenBadge.vue
```

### Avoid

- Global CSS (no `scoped`) except for tokens / reset.
- Tailwind utility soup in templates (wires readability of white-label
  overrides; tokens are the brand surface).
- Inline `style="..."` (already noted).

| Required | Forbidden |
|----------|-----------|
| Semantic HTML (`<button>`, `<nav>`, `<main>`, `<article>`) | `<div onclick>` |
| `aria-label` only when no visible label | Icon CDN covers with no `aria-label` |
| Keyboard handler paired with click handler | Click handler alone |
| Color contrast ≥ 4.5:1 for body text, 3:1 for large text | Contrast ratios that fail WCAG AA |
| `<label for="...">` on form controls | Placeholder as the only label |
| `role="alert"` on error message, `role="status"` on success | `role="alert"` on success |

### Quick check before commit

- [ ] Component renders with `npm run dev` and Chrome 100+.
- [ ] Component works with keyboard alone (Tab / Shift+Tab / Enter / Esc).
- [ ] Component announces state changes (loading, error, success) via
      `role="status"` or `aria-live`.
- [ ] Visible focus ring (browser default or `:focus-visible`).
- [ ] No axe-core violations when run via `vitest-axe` (if Vitest is added
      in T1.22 follow-up).

---

## Common Mistakes

### Mistake 1: business logic in the component

```vue
<script setup lang="ts">
// BAD — saving logic in the component
async function onSave() {
  const response = await fetch('/api/ai-kbs/documents', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(localDoc),
  })
  if (!response.ok) {
    toast.error('Save failed')
    return
  }
  router.push(`/knowledge/${docCode}`)
}
</script>
```

```vue
<script setup lang="ts">
// GOOD — store handles network + error; component handles UI event
const { save } = useKnowledge()
async function onSave() {
  await save(localDoc)
  router.push(`/knowledge/${localDoc.docCode}`)
}
</script>
```

### Mistake 2: store mutation in computed

```vue
<script setup lang="ts">
// BAD — computed mutates
const title = computed(() => {
  docStore.title = docStore.title.trim()   // ← side effect in computed
  return docStore.title
})
```

```vue
<script setup lang="ts">
// GOOD — pure computed; mutation in a watcher or explicit action
const title = computed(() => docStore.title.trim())

watchEffect(() => {
  docStore.setTitle(docStore.title.trim())   // explicit action
})
```

### Mistake 3: prop mutation

```vue
<script setup lang="ts">
// BAD — prop mutated in place
const props = defineProps<{ doc: Document }>()
function onTitleClick() {
  props.doc.title = 'new'   // <-- Vue warning, breaks reactivity
}
</script>
```

```vue
<script setup lang="ts">
// GOOD — emit update; parent owns the state
const props = defineProps<{ doc: Document }>()
const emit = defineEmits<{ (e: 'update:doc', value: Document): void }>()
function onTitleClick() {
  emit('update:doc', { ...props.doc, title: 'new' })
}
</script>
```

### Mistake 4: deep watchers without justification

```vue
<script setup lang="ts">
// BAD — deep watch is expensive and hard to debug
watch(() => docStore.documents, () => { /* runs on any nested change */ },
  { deep: true })
```

```vue
<script setup lang="ts">
// GOOD — watch the specific reactive value you care about
watch(() => docStore.documents.length, (n) => {
  console.log('document count:', n)
})
```

### Mistake 5: not testing the keyboard path

A button that only handles `@click` but has no `@keydown.enter` for
keyboard activation is a real accessibility bug. The pattern:

```vue
<template>
  <button
    type="button"
    @click="onActivate"
    @keydown.enter.prevent="onActivate"
    @keydown.space.prevent="onActivate"
  >
    {{ label }}
  </button>
</template>
```

Or simpler: use `<button type="button">` and let the browser handle
keyboard natively.

---

## Examples

### Good: a list item component

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { type Document } from '@/types/doc'

interface Props {
  doc: Document
  selected?: boolean
}
const props = withDefaults(defineProps<Props>(), { selected: false })

const emit = defineEmits<{
  (e: 'select', docCode: Document['docCode']): void
  (e: 'delete', docCode: Document['docCode']): void
}>()

const store = useKnowledgeStore()
const canDelete = computed(() => store.canDelete(props.doc))

function onSelect() { emit('select', props.doc.docCode) }
function onDelete() { emit('delete', props.doc.docCode) }
</script>

<template>
  <li
    class="doc-item"
    :class="{ 'doc-item--selected': selected }"
    role="option"
    :aria-selected="selected"
  >
    <button type="button" class="doc-item__title" @click="onSelect">
      {{ doc.title }}
    </button>
    <button
      v-if="canDelete"
      type="button"
      class="doc-item__delete"
      :aria-label="`Delete ${doc.title}`"
      @click="onDelete"
    >
      Delete
    </button>
  </li>
</template>

<style scoped>
.doc-item {
  display: flex;
  align-items: center;
  gap: var(--kb-spacing-2);
  padding: var(--kb-spacing-2);
  border-bottom: 1px solid var(--kb-color-border);
}
.doc-item--selected {
  background: var(--kb-color-surface-selected);
}
.doc-item__title {
  flex: 1;
  text-align: left;
  background: transparent;
  border: 0;
  color: var(--kb-color-text);
  cursor: pointer;
}
.doc-item__delete {
  background: transparent;
  border: 1px solid var(--kb-color-border);
  border-radius: var(--kb-radius-sm);
  color: var(--kb-color-danger);
  cursor: pointer;
}
</style>
```

### Bad: the same component done wrong

```vue
<script setup lang="ts">
// BAD — props: any, business logic in component, hardcoded colors
import { ref } from 'vue'
const props = defineProps<{ doc: any }>()             // props: any
const docCode = ref(props.doc.code)                   // assumes structure
function onDelete() {
  fetch(`/api/ai-kbs/documents/${docCode.value}`, { method: 'DELETE' })
    .then(() => alert('deleted'))
}
</script>

<template>
  <li style="padding: 12px; border-bottom: 1px solid #ccc">
    <div @click="alert(props.doc.title)">{{ docCode }}</div>
    <div style="color: red" @click="onDelete">×</div>
  </li>
</template>
```

The bad version has: untyped props, no keyboard handler, hardcoded
colors, no aria, alert() for feedback, fetch() in the component, no
delete confirmation.

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §1 frontend box (api → stores → router)
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3 AC-F4 (white-label scrub), AC-F5 (auth guard)
- `.trellis/spec/frontend/directory-structure.md` — `src/components/` placement
- `.trellis/spec/frontend/state-management.md` — store interaction
- `.trellis/spec/frontend/type-safety.md` — typed props / emits / slots
- `.trellis/spec/frontend/quality-guidelines.md` — lint / test / a11y gates