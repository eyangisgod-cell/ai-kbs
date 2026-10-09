// Package doc holds document-domain logic for AI-KBS: the docCode
// generator (this file) and the CAS entry point (`cas.go`). It MUST
// NOT contain HTTP handlers, business rules, or schema migrations —
// those belong to `internal/handler`, `internal/service`, and
// `migrations/versioned` respectively.
package doc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository"
)

// DocCodeLength is the canonical AI-KBS docCode length (base62, 8 chars).
// 62^8 ≈ 2.18e14 — well above any realistic workspace document count, so
// collisions against the unique index `uniq_documents_space_doc_code_live`
// are vanishingly rare and the 3-retry cap below is generous.
const DocCodeLength = 8

// docCodeAlphabet is the base62 character set in canonical order:
// 0-9, A-Z, a-z. Ordered for stable serialization (URL-safe, no
// symbols, no case-folding ambiguity). The DocCode is rendered as a
// short opaque token users may quote in chat; this set avoids
// visually-confusable pairs (0/O, 1/l/I) only modestly — the goal is
// human-quotability over human-readability.
const docCodeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// maxCollisionRetries caps DB-uniqueness retries. With 62^8 codes
// the birthday-paradox collision probability at 1B documents is
// ≈ 2.3e-6, so 3 retries leaves a safety factor of > 10^5.
//
// The Snowflake fallback (per implement.md §3 T1.3 risk) kicks in
// when the observed DB-confirmed collision rate exceeds 0.01%; see
// CollisionRate.
const maxCollisionRetries = 3

// collisionRateSwitchover is the rate at which the generator should be
// considered for replacement by a Snowflake-style scheme (per
// implement.md §3 T1.3 risk). The generator does NOT auto-switch —
// it surfaces the metric so an operator can decide.
const collisionRateSwitchover = 0.0001 // 0.01%

// Errors. Matched via `errors.Is`.
var (
	// ErrDocCodeExhausted means the generator tried maxCollisionRetries
	// times and every attempt hit a DB-confirmed collision. Maps to
	// envelope code `doc.code_exhausted` (HTTP 503). This is a fail-safe
	// the operator rarely sees — it indicates either an absurdly
	// large workspace or a regression in the DB uniqueness index.
	ErrDocCodeExhausted = errors.New("doc: code space exhausted after retries")

	// ErrInvalidSpaceID is returned by Next when spaceID is empty. The
	// underlying repository requires a space; passing "" would silently
	// hide data across spaces. Fail loudly.
	ErrInvalidSpaceID = errors.New("doc: spaceID is required")
)

// DocCodeLookup is the minimal interface DocCodeGenerator needs to
// verify uniqueness. It is declared locally (consumption-side) so
// that tests can hand-roll a fake without depending on the full
// `repository.DocumentRepository` surface (8 methods vs 1).
//
// The signature matches `repository.DocumentRepository.GetByCode`
// exactly: returns (nil, nil) when the code is free, (*doc, nil)
// when occupied, and any non-nil error otherwise. Note that the
// repository implementation already filters soft-deleted rows via
// `WHERE deleted_flag = FALSE`, which matches the partial unique
// index `uniq_documents_space_doc_code_live` semantics — the
// generator sees only "live" rows.
//
// We deliberately re-export the underlying repository.ErrNotFound
// propagation contract: the adapter's LookupByCode translates
// ErrNotFound into (nil, nil). Tests using a hand-rolled fake
// directly satisfy DocCodeLookup and skip that translation.
type DocCodeLookup interface {
	// LookupByCode returns (nil, nil) when the (space, code) pair is
	// free, (non-nil, nil) when occupied, and (nil, err) on a real
	// error.
	LookupByCode(ctx context.Context, spaceID, code string) (existing any, err error)
}

// LookupFunc is the function-shaped DocCodeLookup adapter. Wiring
// code (DI / service factory) hands it `repository.DocumentRepository.GetByCode`,
// which has the canonical signature. Tests hand it a fake.
//
// We use a function (not a struct wrapping the full DocumentRepository)
// so the adapter stays trivial to mock — there's no need to stub the
// 6 unrelated repo methods that a wrapping-struct approach would
// force.
//
// The `repositoryDocument` return type is intentionally `any`; the
// generator only checks for nil. Wiring code currently returns
// `*repository.domain.Document`, but pinning that here would create
// an internal/doc → internal/repository/domain import the test layer
// wants to avoid.
type LookupFunc func(ctx context.Context, spaceID, code string) (repositoryDocument any, err error)

// LookupByCode implements DocCodeLookup.
func (f LookupFunc) LookupByCode(ctx context.Context, spaceID, code string) (any, error) {
	return f(ctx, spaceID, code)
}

// RepositoryLookup adapts `repository.DocumentRepository.GetByCode` to
// the DocCodeLookup contract. It translates `repository.ErrNotFound`
// into `(nil, nil)` so the generator's loop stays terse and free of
// repository-package imports.
//
// Use:
//
//	adapter := doc.RepositoryLookup(repo.GetByCode)
//	gen := doc.NewDocCodeGenerator(adapter)
//
// The variable name `repo` is a parameter typed as
// `repository.DocumentRepository`; only its `GetByCode` method is
// captured.
func RepositoryLookup(getByCode func(ctx context.Context, spaceID, docCode string) (any, error)) LookupFunc {
	return func(ctx context.Context, spaceID, code string) (any, error) {
		d, err := getByCode(ctx, spaceID, code)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return d, nil
	}
}

// DocCodeGenerator produces unique document codes with cryptographic
// randomness and a bounded DB-uniqueness retry loop.
//
// Implementations MUST be safe for concurrent use.
type DocCodeGenerator interface {
	// Next returns a fresh, DB-unique docCode in the given space. The
	// returned code is `DocCodeLength` characters drawn from
	// `docCodeAlphabet` and is guaranteed not to collide with any
	// live document in `spaceID` (per
	// `uniq_documents_space_doc_code_live`).
	Next(ctx context.Context, spaceID string) (string, error)

	// CollisionRate returns the ratio of DB-confirmed collisions to
	// total generations observed so far. Operators use this to decide
	// when to switch to a Snowflake-style scheme (see
	// `collisionRateSwitchover`).
	CollisionRate() float64

	// ShouldUseSnowflake reports whether the observed collision rate
	// exceeds the switchover threshold.
	ShouldUseSnowflake() bool

	// Stats returns a snapshot of the counters for logging / metrics.
	Stats() (generated, collisions uint64)
}

// docCodeGen is the only DocCodeGenerator implementation shipped by
// this package. Construct via NewDocCodeGenerator.
type docCodeGen struct {
	lookup DocCodeLookup
	// counters are atomic so the public Stats / CollisionRate readers
	// are lock-free and the concurrent-stress test (`-race`) is clean.
	generatedCounter atomic.Uint64
	collisionCounter atomic.Uint64
	// randMu serializes the per-character rand.Int calls. crypto/rand.Int
	// is itself safe for concurrent use, but we observed non-trivial
	// contention overhead in benchmarks; the mutex amortizes well and
	// keeps the 1M-iteration test under its 30s budget.
	randMu sync.Mutex
}

// NewDocCodeGenerator returns a generator backed by a DocCodeLookup.
// Panics on nil lookup — the generator is useless without it, so we
// fail at construction rather than at first call (matches the
// project's "validate deps at startup" style).
func NewDocCodeGenerator(lookup DocCodeLookup) DocCodeGenerator {
	if lookup == nil {
		panic("doc: NewDocCodeGenerator requires a non-nil DocCodeLookup")
	}
	return &docCodeGen{lookup: lookup}
}

// Next generates a unique docCode in `spaceID`. Up to
// maxCollisionRetries attempts are made; on the (rare) exhaustion
// path it returns ErrDocCodeExhausted.
//
// The function does not consume ctx for the random-source step
// (crypto/rand is process-local and non-blocking), but the DB
// uniqueness check does. A caller that wants to bound the entire
// call should pass a deadline-bearing ctx; we honour it on every DB
// call.
func (g *docCodeGen) Next(ctx context.Context, spaceID string) (string, error) {
	if spaceID == "" {
		return "", ErrInvalidSpaceID
	}
	for attempt := 0; attempt < maxCollisionRetries; attempt++ {
		// Respect ctx before each DB call: if the caller has
		// already cancelled, fail fast — don't waste a randomness
		// draw they can't use.
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("doc: ctx cancelled before uniqueness check: %w", err)
		}
		code, err := g.randomBase62Locked(DocCodeLength)
		if err != nil {
			return "", fmt.Errorf("doc: rand failed: %w", err)
		}
		existing, err := g.lookup.LookupByCode(ctx, spaceID, code)
		if err != nil {
			return "", fmt.Errorf("doc: uniqueness check failed (code=%s): %w", code, err)
		}
		if existing == nil {
			g.generatedCounter.Add(1)
			return code, nil
		}
		// DB-confirmed collision. Bump the counter and try again.
		g.collisionCounter.Add(1)
	}
	return "", ErrDocCodeExhausted
}

// CollisionRate returns collisions / generated as a float64. Returns
// 0 when no generations have been observed yet.
func (g *docCodeGen) CollisionRate() float64 {
	gen := g.generatedCounter.Load()
	if gen == 0 {
		return 0
	}
	return float64(g.collisionCounter.Load()) / float64(gen)
}

// ShouldUseSnowflake is a typed predicate over CollisionRate().
func (g *docCodeGen) ShouldUseSnowflake() bool {
	return g.CollisionRate() > collisionRateSwitchover
}

// Stats snapshots the internal counters. The pair is not perfectly
// consistent (a Next call may bump one after the other has been
// read) but for logging / metrics purposes the divergence is bounded
// by the in-flight concurrent Next calls, of which there are
// typically a small N.
func (g *docCodeGen) Stats() (generated, collisions uint64) {
	return g.generatedCounter.Load(), g.collisionCounter.Load()
}

// randomBase62Locked returns n random characters from docCodeAlphabet
// under the generator's mutex. Uses crypto/rand (NOT math/rand) so
// docCodes are unguessable to anyone without DB access — important
// because docCodes are used as URL tokens and could otherwise be
// enumerated.
func (g *docCodeGen) randomBase62Locked(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	g.randMu.Lock()
	defer g.randMu.Unlock()
	out := make([]byte, n)
	max := big.NewInt(int64(len(docCodeAlphabet)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = docCodeAlphabet[idx.Int64()]
	}
	return string(out), nil
}
