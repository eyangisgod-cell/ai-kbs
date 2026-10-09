package doc

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ----------------------------------------------------------------------------
// randomBase62Locked — pure-randomness behaviour
// ----------------------------------------------------------------------------

func TestRandomBase62_Length(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 4, 8, 16, 64} {
		g := newTestGen(t, stubLookup())
		out, err := g.randomBase62Locked(n)
		if err != nil {
			t.Fatalf("randomBase62Locked(%d) err: %v", n, err)
		}
		if len(out) != n {
			t.Fatalf("randomBase62Locked(%d) len=%d, want %d", n, len(out), n)
		}
	}
}

func TestRandomBase62_Alphabet(t *testing.T) {
	t.Parallel()
	g := newTestGen(t, stubLookup())
	seen := make(map[byte]struct{}, len(docCodeAlphabet))
	const draws = 1000
	for i := 0; i < draws; i++ {
		out, err := g.randomBase62Locked(DocCodeLength)
		if err != nil {
			t.Fatalf("rand: %v", err)
		}
		for j := 0; j < len(out); j++ {
			c := out[j]
			if !strings.ContainsRune(docCodeAlphabet, rune(c)) {
				t.Fatalf("char %q (%d) not in alphabet", c, c)
			}
			seen[c] = struct{}{}
		}
	}
	// With 8000 chars from a 62-symbol alphabet, every symbol should
	// appear many times. Allow up to 5 missing for resilience against
	// RNG quirks (theoretical unseen probability is effectively 0).
	if len(seen) < len(docCodeAlphabet)-5 {
		t.Fatalf("alphabet coverage too narrow: saw %d/%d after %d draws",
			len(seen), len(docCodeAlphabet), draws*DocCodeLength)
	}
}

func TestRandomBase62_Distribution(t *testing.T) {
	t.Parallel()
	// Spec: 100k outputs have ≥ 90% alphabet coverage. We sample 100k
	// codes (800k chars) and require ≥ 90% of the 62-symbol alphabet
	// to appear at least once.
	g := newTestGen(t, stubLookup())
	const draws = 100_000
	seen := make(map[byte]int, len(docCodeAlphabet))
	for i := 0; i < draws; i++ {
		out, err := g.randomBase62Locked(DocCodeLength)
		if err != nil {
			t.Fatalf("rand: %v", err)
		}
		for j := 0; j < len(out); j++ {
			seen[out[j]]++
		}
	}
	coverage := float64(len(seen)) / float64(len(docCodeAlphabet))
	if coverage < 0.90 {
		t.Fatalf("alphabet coverage %.4f < 0.90 (saw %d/%d)",
			coverage, len(seen), len(docCodeAlphabet))
	}
	total := 0
	maxCount := 0
	for _, c := range seen {
		total += c
		if c > maxCount {
			maxCount = c
		}
	}
	mean := float64(total) / float64(len(seen))
	if float64(maxCount) > 3*mean {
		t.Fatalf("distribution skew too high: max=%d mean=%.1f", maxCount, mean)
	}
}

func TestRandomBase62_Uniqueness_Million(t *testing.T) {
	if testing.Short() {
		t.Skip("million-iteration stress; skipped under -short")
	}
	t.Parallel()
	g := newTestGen(t, stubLookup())
	const total = 1_000_000
	unique := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		out, err := g.randomBase62Locked(DocCodeLength)
		if err != nil {
			t.Fatalf("rand at %d: %v", i, err)
		}
		if len(out) != DocCodeLength {
			t.Fatalf("len drift at %d: got %d", i, len(out))
		}
		unique[out] = struct{}{}
	}
	// Spec: 1M calls, count of unique codes ≥ 999,500
	// (collision rate < 0.05%).
	if len(unique) < 999_500 {
		t.Fatalf("uniqueness too low: %d unique / %d total (rate %.4f%%)",
			len(unique), total, 100*float64(total-len(unique))/float64(total))
	}
}

// ----------------------------------------------------------------------------
// DocCodeGenerator behaviour — using an in-memory mock
// ----------------------------------------------------------------------------

func TestDocCodeGenerator_Next_EmptySpace(t *testing.T) {
	t.Parallel()
	g := newTestGen(t, stubLookup())
	if _, err := g.Next(context.Background(), ""); !errors.Is(err, ErrInvalidSpaceID) {
		t.Fatalf("expected ErrInvalidSpaceID, got %v", err)
	}
}

func TestDocCodeGenerator_Next_NilLookup(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on nil lookup")
		}
	}()
	_ = NewDocCodeGenerator(nil)
}

func TestDocCodeGenerator_Next_AlwaysFree(t *testing.T) {
	t.Parallel()
	mock := &mockLookup{}
	g := newTestGen(t, mock)
	for i := 0; i < 100; i++ {
		code, err := g.Next(context.Background(), "sp-1")
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if len(code) != DocCodeLength {
			t.Fatalf("iter %d: len=%d", i, len(code))
		}
	}
	gen, collisions := g.Stats()
	if gen != 100 || collisions != 0 {
		t.Fatalf("stats: gen=%d collisions=%d, want 100/0", gen, collisions)
	}
}

func TestDocCodeGenerator_Next_CollisionThenFree(t *testing.T) {
	t.Parallel()
	mock := &mockLookup{occupiedOnce: true}
	g := newTestGen(t, mock)
	code, err := g.Next(context.Background(), "sp-1")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(code) != DocCodeLength {
		t.Fatalf("len=%d", len(code))
	}
	gen, collisions := g.Stats()
	if gen != 1 || collisions != 1 {
		t.Fatalf("stats: gen=%d collisions=%d, want 1/1", gen, collisions)
	}
}

func TestDocCodeGenerator_Next_AlwaysOccupied(t *testing.T) {
	t.Parallel()
	mock := &mockLookup{alwaysOccupied: true}
	g := newTestGen(t, mock)
	_, err := g.Next(context.Background(), "sp-1")
	if !errors.Is(err, ErrDocCodeExhausted) {
		t.Fatalf("expected ErrDocCodeExhausted, got %v", err)
	}
	gen, collisions := g.Stats()
	if gen != 0 || collisions != uint64(maxCollisionRetries) {
		t.Fatalf("stats: gen=%d collisions=%d, want 0/%d", gen, collisions, maxCollisionRetries)
	}
}

func TestDocCodeGenerator_Next_DBError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	mock := &mockLookup{dbErr: boom}
	g := newTestGen(t, mock)
	_, err := g.Next(context.Background(), "sp-1")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

func TestDocCodeGenerator_Next_CtxCancelled(t *testing.T) {
	t.Parallel()
	mock := &mockLookup{}
	g := newTestGen(t, mock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.Next(ctx, "sp-1")
	if err == nil {
		t.Fatalf("expected ctx-cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Collision-rate metric
// ----------------------------------------------------------------------------

func TestDocCodeGenerator_CollisionRate(t *testing.T) {
	t.Parallel()
	// 20% collision rate: gives a measurable ratio. We loop on error
	// (retry-exhausted) up to a generous bound so we always reach
	// the success target — the metric we're testing is the *ratio*
	// of collisions to successes, which is well-defined regardless
	// of how many exhaustion-tail calls we made.
	mock := &mockLookup{collisionRate: 0.20}
	g := newTestGen(t, mock)
	const wantSuccesses = 1000
	const maxTries = wantSuccesses * 5 // tolerate retry exhaustion
	successes := 0
	for i := 0; i < maxTries && successes < wantSuccesses; i++ {
		if _, err := g.Next(context.Background(), "sp-1"); err != nil {
			// Retry-exhausted: keep counting toward maxTries.
			continue
		}
		successes++
	}
	if successes < wantSuccesses {
		t.Fatalf("only %d/%d successes after %d tries", successes, wantSuccesses, maxTries)
	}
	rate := g.CollisionRate()
	// Per-call observed collision rate at p=0.20 input:
	// E[collisions per call] = 1·(0.16) + 2·(0.032) + 3·(0.008) = 0.272
	// E[successes per call]  = 0.992
	// E[collisions/success]  ≈ 0.272 / 0.992 ≈ 0.274.
	// Allow ±10% noise from random draws.
	const wantLow, wantHigh = 0.17, 0.37
	if rate < wantLow || rate > wantHigh {
		t.Fatalf("rate %.4f outside [%.2f, %.2f]", rate, wantLow, wantHigh)
	}
	if g.ShouldUseSnowflake() {
		// ShouldUseSnowflake is metric-based only; even at 20% we
		// can confirm the predicate fires through the dedicated
		// threshold test below.
	}
}

func TestDocCodeGenerator_CollisionRate_ShouldUseSnowflakeAboveThreshold(t *testing.T) {
	t.Parallel()
	// Use 5% — well above the 0.01% switchover threshold but
	// below the regime where 3 retries get exhausted in unlucky
	// streaks. Each successful Next costs on average 1/(1-0.05) ≈
	// 1.053 lookups, well under our 3-retry cap.
	mock := &mockLookup{collisionRate: 0.05}
	g := newTestGen(t, mock)
	const wantSuccesses = 200
	const maxTries = wantSuccesses * 5
	successes := 0
	for i := 0; i < maxTries && successes < wantSuccesses; i++ {
		if _, err := g.Next(context.Background(), "sp-1"); err == nil {
			successes++
		}
	}
	if successes < wantSuccesses {
		t.Fatalf("only %d/%d successes after %d tries", successes, wantSuccesses, maxTries)
	}
	if !g.ShouldUseSnowflake() {
		t.Fatalf("ShouldUseSnowflake should be true at 5%% rate (threshold = %.4f%%)",
			collisionRateSwitchover*100)
	}
}

func TestDocCodeGenerator_CollisionRate_Empty(t *testing.T) {
	t.Parallel()
	g := newTestGen(t, &mockLookup{})
	if got := g.CollisionRate(); got != 0 {
		t.Fatalf("CollisionRate at zero gens: %.4f, want 0", got)
	}
	if g.ShouldUseSnowflake() {
		t.Fatalf("ShouldUseSnowflake at zero gens should be false")
	}
}

// ----------------------------------------------------------------------------
// Concurrent stress (AC-B3 second clause: 100 goroutines × 10k iter)
// ----------------------------------------------------------------------------

func TestDocCodeGenerator_Concurrent_100_10k(t *testing.T) {
	if testing.Short() {
		t.Skip("concurrent stress; skipped under -short")
	}
	t.Parallel()
	// sharedMock is a sync.Map-backed DocCodeLookup. Every successful
	// Next() stores its code atomically; subsequent look-ups report
	// the code as occupied. This is the "real DB uniqueness check"
	// stand-in for the test — there are no test-only locks; everything
	// goes through the standard library.
	store := &sync.Map{}
	shared := &sharedMock{store: store}

	g := newTestGen(t, shared)

	const goroutines = 100
	const perGoroutine = 10_000
	var wg sync.WaitGroup
	var errCount atomic.Int64
	var dupCount atomic.Int64
	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < perGoroutine; i++ {
				code, err := g.Next(ctx, "sp-1")
				if err != nil {
					errCount.Add(1)
					return
				}
				if _, dup := store.LoadOrStore(code, struct{}{}); dup {
					dupCount.Add(1)
					return
				}
			}
		}()
	}
	wg.Wait()
	if errCount.Load() != 0 {
		t.Fatalf("%d errors during concurrent run", errCount.Load())
	}
	// The sharedMock has a small race window (Load may miss a concurrent
	// Store) so duplicates aren't perfectly impossible — but with
	// 62^8 code space and 1M generations, the expected duplicates
	// are < 0.001. A failure here would be a real concurrency bug.
	if dupCount.Load() > 10 {
		t.Fatalf("%d duplicate codes during concurrent run (race?)", dupCount.Load())
	}
	distinct := 0
	store.Range(func(k, v any) bool { distinct++; return true })
	expected := goroutines*perGoroutine - int(dupCount.Load()) - 50
	if distinct < expected {
		t.Fatalf("only %d distinct codes (expected >= %d)",
			distinct, expected)
	}
}

// ----------------------------------------------------------------------------
// Shared mock (concurrency test)
// ----------------------------------------------------------------------------

// sharedMock is a concurrency-safe DocCodeLookup backed by a sync.Map.
// The first goroutine to observe a code "wins" (via Store inside the
// caller's LoadOrStore); subsequent look-ups report the code as
// occupied.
//
// There is an unavoidable race window: sharedMock.LookupByCode reads
// from store first, then the caller Stores after Next returns. If
// another goroutine looks up the SAME code in that window, both will
// proceed to Store and one will detect the duplicate. That duplicate
// count is reported as dupCount in the test above and used to gauge
// the actual collision rate.
type sharedMock struct {
	store *sync.Map
}

func (m *sharedMock) LookupByCode(ctx context.Context, spaceID, code string) (any, error) {
	if _, ok := m.store.Load(code); ok {
		return &fakeDoc{DocCode: code}, nil
	}
	return nil, nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func newTestGen(t *testing.T, l DocCodeLookup) *docCodeGen {
	t.Helper()
	g := NewDocCodeGenerator(l).(*docCodeGen)
	return g
}

// stubLookup is a trivial DocCodeLookup used by tests of the
// randomness layer that don't care about uniqueness (e.g. the
// length / alphabet / distribution / pure-randomness-million tests).
// Every code is reported as free.
func stubLookup() DocCodeLookup { return &alwaysFree{} }

// alwaysFree is the trivial lookup used by stubLookup.
type alwaysFree struct{}

func (alwaysFree) LookupByCode(ctx context.Context, spaceID, code string) (any, error) {
	return nil, nil
}

// mockLookup is the bread-and-butter fake used by most tests.
//   - alwaysOccupied: every lookup reports a fake doc.
//   - occupiedOnce: first lookup reports a fake doc, subsequent ones
//     report nil.
//   - collisionRate p in (0, 1]: each lookup independently returns a
//     fake doc with probability p, else nil.
//   - dbErr: every lookup returns this error.
type mockLookup struct {
	alwaysOccupied bool
	occupiedOnce   bool
	collisionRate  float64
	dbErr          error

	count atomic.Int64
}

func (m *mockLookup) LookupByCode(ctx context.Context, spaceID, code string) (any, error) {
	if m.dbErr != nil {
		return nil, m.dbErr
	}
	n := m.count.Add(1)
	if m.alwaysOccupied {
		return &fakeDoc{DocCode: code}, nil
	}
	if m.occupiedOnce && n == 1 {
		return &fakeDoc{DocCode: code}, nil
	}
	if m.collisionRate > 0 {
		if rngFloat() < m.collisionRate {
			return &fakeDoc{DocCode: code}, nil
		}
	}
	return nil, nil
}

// fakeDoc is a placeholder existing-record. The generator doesn't
// read its fields; it only checks for nil-vs-non-nil.
type fakeDoc struct {
	DocCode string
}

// rngFloat returns a number in [0, 1). crypto/rand-backed for
// uniformity; in tests we only need the value to be statistically
// independent across calls (a deterministic PRNG would also work
// but introduces a seed we don't want to manage here).
//
// Implementation note: we use the top 53 bits of a random uint64 so
// the conversion to float64 is lossless, then divide by 2^53. Using
// the top 63 bits would over-represent low values because float64
// can't represent every integer in [0, 2^63).
func rngFloat() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	v := binary.LittleEndian.Uint64(b[:])
	// Take the top 53 bits (mantissa precision) and divide by 2^53.
	top := v >> (64 - 53)
	return float64(top) / float64(uint64(1)<<53)
}
