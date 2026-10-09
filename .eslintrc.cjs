// AI-KBS root ESLint configuration (placeholder)
//
// STATUS: DORMANT in v1. Frontend linting is intentionally NOT enabled
// in CI for the v1 GA milestone (implement.md §2 T0.5). The T1.22
// frontend scaffold owner should fill this in when ESLint becomes
// part of `make lint`. The reason v1 ships without frontend lint:
//   1. T1.22 (frontend scaffold) is a follow-up task; we don't want
//      to invent rules for code that doesn't exist yet.
//   2. The forked WeKnora Vue 3 SPA ships its own lint assumptions
//      that may or may not match ai-kbs conventions; better to wait
//      until T1.23 (white-label scrub) is done so the lint baseline
//      reflects ai-kbs-owned code.
//
// To activate: uncomment the `root` + `rules` block below and add
// `eslint . --ext .ts,.vue` to Makefile's `lint-frontend` target.
//
// module.exports = {
//   root: true,
//   env: { browser: true, es2022: true, node: true },
//   extends: [
//     'eslint:recommended',
//     'plugin:@typescript-eslint/recommended',
//     'plugin:vue/vue3-recommended',
//   ],
//   parserOptions: { ecmaVersion: 2022, sourceType: 'module' },
//   ignorePatterns: ['dist/', 'node_modules/', '**/*.d.ts'],
//   rules: {
//     'no-console': ['warn', { allow: ['warn', 'error'] }],
//     'vue/multi-word-component-names': 'off', // WeKnora fork uses single-word routes
//   },
// };
