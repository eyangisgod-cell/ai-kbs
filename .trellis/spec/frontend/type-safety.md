# Type Safety

> How TypeScript is used in `frontend/` for compile-time + runtime safety.

---

## Overview

AI-KBS uses **TypeScript with strict mode** for the entire frontend.
The compile-time types are an **invariant** of the codebase — every
`.ts` / `.tsx` / `.vue` script block type-checks under
`tsc --noEmit` (CI gate). The build fails if any new code introduces
a `any`, an unchecked cast, or an unsafe property access.

Type safety is layered: **runtime validation at the boundary** (HTTP
in), **branded IDs everywhere** (no `string` for `docCode`),
**strict compile-time** everywhere else. We do NOT adopt a TypeScript
type that requires `// @ts-expect-error` or `as any` to bypass —
those are code review rejections.

Three principles:

1. **Parse, don't trust.** Every value that crosses an HTTP boundary is
   validated by Zod before it enters the store. The TypeScript type is
   a hint; Zod is the contract.
2. **Brands for things that should never mix up.** A `DocCode` is not a
   string; it's a `string & { __brand: 'DocCode' }`. The compiler
   refuses to let `UserId` flow where `DocCode` is expected.
3. **Inference over declaration.** Where the compiler can do the work,
   let it. Annotations are for public surfaces (function params, exported
   types, component props), not for local variables that already
   inherit their type from `ref()` / `computed()`.

---

## Type Organization

### Where types live

| Type | Lives in | Why |
|------|----------|-----|
| Backend payload mirror | `src/types/<resource>.ts` | Single source of truth; mirrors backend types |
| Branded IDs | `src/types/branded.ts` | Reused across stores, components, api |
| Envelope | `src/types/envelope.ts` | Mirrors backend envelope v1.0 |
| Component-local types | Inline in the `<script setup>` block | Not exported; not reused |
| Composable return types | Inline (inferred) | Consumers see the inferred shape |
| Store action param types | Inline | Each action's input is one-shot |
| Library re-exports | `src/types/lib-<vendor>.d.ts` | Ambient declarations for libraries without types |

### File layout

```
src/types/
├── envelope.ts        # mirrors backend envelope v1.0
├── doc.ts             # Document, DocumentCreateRequest, DocumentSummary
├── binding.ts         # Binding, BindingSummary
├── knowledge-base.ts  # KnowledgeBase, KnowledgeBaseCreate
├── user.ts            # User, LoginRequest, LoginResponse
├── chat.ts            # ChatMessage, ChatRequest, ChatResponse (SSE chunks)
├── agent.ts           # AgentConfig, AgentRun
├── branded.ts         # DocCode, Version, UserId, BindingId, TenantId
├── error.ts           # ErrorEnvelope (from envelope.error)
└── index.ts           # re-exports for ergonomic `import { ... } from '@/types'`
```

### Public vs private

- An exported type from `src/types/` is part of the contract; changing
  it is a breaking change for stores and components.
- A type declared inline in `<script setup>` is private; refactor freely.
- A type declared inline in a `stores/<domain>.ts` and exported is part
  of the store's public API — components depend on it.

### Co-located vs separate

- A type used by ONE component AND its tests lives inline in the
  component file.
- A type used by ≥ 2 components OR by a store + a component lives in
  `src/types/`.
- A type that mirrors a backend payload lives in `src/types/<resource>.ts`
  even if only one place uses it (so the next consumer finds it
  immediately).

---

## Validation (Runtime)

Compile-time types are erased at runtime. Anything that crosses an HTTP
boundary MUST be validated by **Zod** before it enters the store.

### Pattern: parse at the boundary, type internally

```ts
// src/api/knowledge-base.ts
import { z } from 'zod'
import { KnowledgeBaseSchema } from '@/types/knowledge-base'

export async function list(): Promise<KnowledgeBase[]> {
  const raw = await http.get<unknown>('/api/ai-kbs/knowledge-base')
  return z.array(KnowledgeBaseSchema).parse(raw)   // throws on mismatch
}
```

The schema is exported from `src/types/<resource>.ts` so the same
schema can be reused in the store, the form, and tests.

```ts
// src/types/knowledge-base.ts
import { z } from 'zod'

export const KnowledgeBaseSchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1).max(120),
  description: z.string().max(2000).optional(),
  createdAt: z.string().datetime(),
  updatedAt: z.string().datetime(),
  docCount: z.number().int().nonnegative(),
})

export type KnowledgeBase = z.infer<typeof KnowledgeBaseSchema>
```

The TypeScript type is **derived from Zod**, not the other way around.
Zod is the source of truth; TypeScript is the convenience.

### Where validation happens

| Boundary | Validator |
|----------|-----------|
| HTTP response (axios interceptor or per-call) | `z.array(X).parse(response)` |
| HTTP request body (form submission) | `X.parse(formValue)` |
| localStorage read (persisted store hydration) | `X.parse(JSON.parse(raw))` |
| URL query param | `X.parse(query.value)` |
| SSE / WebSocket message | per-message schema |

### Error reporting

If a parse fails, the helper converts to `ErrorEnvelope`:

```ts
function toErrorEnvelope(err: unknown, code = 'frontend.parse'): ErrorEnvelope {
  if (err instanceof z.ZodError) {
    return {
      code,
      message: `Schema mismatch: ${err.issues.length} field(s)`,
      issues: err.issues.map(i => ({ path: i.path.join('.'), message: i.message })),
    }
  }
  return { code: 'frontend.unknown', message: String(err) }
}
```

The error is logged with a "schema mismatch" warning so they show up
in observability, not just in user-facing toasts.

---

## Branded Types

`string` is too loose. A `DocCode`, `UserId`, `BindingId`, `Version`
are all strings at runtime, but mixing them is a bug:

```ts
// without branding
function getDoc(docCode: string) { ... }
const userId = 'u_abc'
getDoc(userId)   // ← compiles; nonsense
```

Branded types make the compiler refuse this:

```ts
// src/types/branded.ts
type Brand<K, T> = K & { readonly __brand: T }

export type DocCode   = Brand<string, 'DocCode'>
export type UserId    = Brand<string, 'UserId'>
export type BindingId = Brand<string, 'BindingId'>
export type TenantId  = Brand<string, 'TenantId'>
export type Version   = Brand<number, 'Version'>   // 64-bit monotonic

// Constructor helpers (the only way to mint a branded value)
export const DocCode   = (s: string): DocCode   => s as DocCode
export const UserId    = (s: string): UserId    => s as UserId
export const BindingId = (s: string): BindingId => s as BindingId
export const TenantId  = (s: string): TenantId  => s as TenantId
export const Version   = (n: number): Version   => n as Version
```

### Where to brand

| Concept | Brand? | Why |
|---------|--------|-----|
| `docCode` | Yes | Mix-up with `bindingId` is a real bug |
| `userId` | Yes | Mix-up with `tenantId` is a real bug |
| `version` | Yes | Distinct from generic `number`; prevents int/string mix-up |
| `email` | No | Already validated; overlap with `string` is rare |
| `url`, `path`, `query` | No | Validated, low-mixup surface |
| Free-form text | No | Branding everything is busywork |

### When to validate

- Brand at the **boundary** (Zod schema → brand constructor).
- Inside the app, use the brand. `as DocCode` outside the constructor
  is a code review rejection.

```ts
// src/api/doc.ts
const docCode = z.string().regex(/^[A-Za-z0-9]{8}$/).transform(DocCode)

const DocSchema = z.object({
  docCode,
  title: z.string().min(1).max(500),
  version: z.number().int().positive().transform(Version),
})
```

---

## Common Patterns

### Discriminated unions for state machines

```ts
// stores/auth.ts — discriminated login state
type LoginState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'error'; error: ErrorEnvelope }
  | { status: 'success'; user: User }
```

Pattern-matching with exhaustive `switch`:

```ts
function render(s: LoginState): string {
  switch (s.status) {
    case 'idle':         return 'Sign in'
    case 'submitting':   return 'Signing in...'
    case 'error':        return s.error.message
    case 'success':      return `Welcome, ${s.user.name}`
  }
  // exhaustiveness check
  const _exhaustive: never = s
  throw new Error(`Unhandled state: ${JSON.stringify(s)}`)
}
```

### Type guards for parsed payloads

```ts
function isDocument(value: unknown): value is Document {
  return DocumentSchema.safeParse(value).success
}
```

`isDocument` is preferred when the caller needs a quick boolean; Zod
parse is preferred when the caller needs the parsed value or the
specific error.

### Utility types

| Utility | Use |
|---------|-----|
| `Partial<T>` | Optional form of a model |
| `Required<T>` | All fields mandatory |
| `Pick<T, K>` | Subset of fields |
| `Omit<T, K>` | Remove fields (e.g., to exclude `__brand`) |
| `Readonly<T>` | Immutable view (Pinia getters, props) |
| `NonNullable<T>` | Strip `null` and `undefined` |
| `ReturnType<typeof fn>` | Avoid re-declaring function signatures |

### Generics for reusable shapes

```ts
// composables/usePagination.ts
export interface UsePaginationOptions<T> {
  fetcher: (page: number, pageSize: number) => Promise<{ items: T[]; total: number }>
  initialPageSize?: number
}
export function usePagination<T>(opts: UsePaginationOptions<T>) {
  // ... typed
}
```

---

## Forbidden Patterns

| Forbidden | Why | Use instead |
|-----------|-----|-------------|
| `any` | Defeats the type system; hides bugs | `unknown` + Zod parse, or a precise interface |
| `as any` (escape hatch) | Same as above; usually a sign the type is wrong | Narrow the source type or wrap with Zod |
| `<T as unknown as U>` double-cast | Hides an incompatible contract | Add a Zod parse or fix the upstream type |
| `// @ts-expect-error` | Almost always fixable in 5 minutes | Fix the underlying type |
| `// @ts-ignore` | Same; use `@ts-expect-error` if the line truly is wrong | Fix the underlying type |
| `JSON.parse` without schema | Bypasses validation | Wrap in `Schema.parse(JSON.parse(s))` |
| Reading `localStorage` directly | Bypasses persistence contract | Go through the store's persisted state |
| Implicit `any` from missing return | Indicates unclear function | Annotate the return type |
| Optional chaining as a substitute for null-checking | Hides bugs deeper in the call chain | Explicit early return; `assertDefined(x)` |

### Anti-pattern examples

```ts
// BAD — any in a function signature
async function saveDoc(doc: any) {
  return api.docs.create(doc)
}

// GOOD — typed request
async function saveDoc(doc: DocumentCreateRequest) {
  return api.docs.create(doc)
}

// BAD — unchecked JSON.parse
const user = JSON.parse(localStorage.getItem('user') || 'null')
console.log(user.name)   // ← crashes if user is null

// GOOD — validate on read
const raw = localStorage.getItem('aikbs.auth')
const user = raw ? UserSchema.parse(JSON.parse(raw)) : null
```

---

## Examples

### Good: branded types flowing through the app

```ts
// src/api/doc.ts
export const DocSchema = z.object({
  docCode: z.string().regex(/^[A-Za-z0-9]{8}$/).transform(DocCode),
  title: z.string().min(1).max(500),
  content: z.string(),
  version: z.number().int().positive().transform(Version),
  updatedAt: z.string().datetime(),
})
export type Document = z.infer<typeof DocSchema>

export const docApi = {
  async get(docCode: DocCode): Promise<Document> {
    return DocSchema.parse(await http.get<unknown>(`/api/ai-kbs/documents/${docCode}`))
  },
  async list(): Promise<Document[]> {
    return z.array(DocSchema).parse(await http.get<unknown>('/api/ai-kbs/documents'))
  },
}

// stores/docs.ts
export const useDocsStore = defineStore('docs', () => {
  const items = ref<Document[]>([])

  async function load() {
    items.value = await docApi.list()
  }

  function findByCode(code: DocCode): Document | undefined {
    return items.value.find(d => d.docCode === code)   // ← no UserId-vs-DocCode mix-up
  }

  return { items, load, findByCode }
})
```

### Good: form input validated on submit

```vue
<script setup lang="ts">
import { z } from 'zod'

const FormSchema = z.object({
  title: z.string().min(1, 'Title required').max(500),
  body: z.string().min(1, 'Body required'),
})

const draft = ref<z.infer<typeof FormSchema>>({ title: '', body: '' })
const errors = ref<Record<string, string>>({})

function onSubmit() {
  const result = FormSchema.safeParse(draft.value)
  if (!result.success) {
    errors.value = Object.fromEntries(
      result.error.issues.map(i => [i.path.join('.'), i.message])
    )
    return
  }
  errors.value = {}
  // submit result.data
}
</script>
```

---

## Related

- `.trellis/tasks/10-09-M1-backend-cli/design.md` §3.1 envelope (mirrored in `types/envelope.ts`)
- `.trellis/tasks/10-09-M1-backend-cli/prd.md` §3.3 AC-F1 (type-check passes), AC-F5 (typed store)
- `.trellis/spec/frontend/state-management.md` — store types
- `.trellis/spec/frontend/component-guidelines.md` — typed props/emits
- `.trellis/spec/frontend/hook-guidelines.md` — typed composables
- `frontend/tsconfig.json` — strict mode flags