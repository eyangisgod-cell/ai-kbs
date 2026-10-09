// Package domain holds the AI-KBS-flavored domain types persisted by
// `internal/repository`. They live in a sub-package (not in the
// repository package itself) so that:
//
//   - the repository package can be imported by tools that need the
//     repos but not the types (rare, but it keeps imports tidy);
//   - the types can be referenced by non-repository callers (handlers,
//     services) without dragging the entire repository graph in.
//
// Fork-compat: existing `internal/types` already defines a `Chunk`,
// `User`, etc. for the WeKnora fork. These AI-KBS types have the same
// names but represent a *different* domain (the 000079 schema), so
// callers MUST import this package explicitly to disambiguate.
package domain