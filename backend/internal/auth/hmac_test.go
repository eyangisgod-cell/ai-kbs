package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- HMAC sign/verify positive cases ----------

func TestHMACSignString_Shape(t *testing.T) {
	got := SignString("GET", "/api/v1/foo", "1760000000", "nonce-abc", "body")
	want := "GET\n/api/v1/foo\n1760000000\nnonce-abc\nbody"
	if got != want {
		t.Fatalf("SignString layout mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestHMACSignString_EmptyBodyOK(t *testing.T) {
	got := SignString("POST", "/api/v1/x", "100", "n", "")
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected newline-separated payload, got %q", got)
	}
	if strings.Count(got, "\n") != 4 {
		t.Fatalf("expected 4 separators, got %d in %q", strings.Count(got, "\n"), got)
	}
}

// TestHMACVerify_RoundTrip covers the happy path across methods.
func TestHMACVerify_RoundTrip(t *testing.T) {
	secret := []byte("supersecret")
	keyID := "key-1"
	method := "GET"
	path := "/api/v1/tenants/42"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-xyz"
	body := []byte(`{"a":1}`)

	sig := ComputeSignature(secret, keyID, method, path, ts, nonce, body)
	if err := VerifySignature(secret, keyID, method, path, ts, nonce, body, sig); err != nil {
		t.Fatalf("VerifySignature(RoundTrip) failed: %v", err)
	}
}

func TestHMACVerify_DifferentMethodRejected(t *testing.T) {
	secret := []byte("k")
	sig := ComputeSignature(secret, "k1", "GET", "/x", "100", "n", nil)
	if err := VerifySignature(secret, "k1", "POST", "/x", "100", "n", nil, sig); err == nil {
		t.Fatal("expected mismatch error when method changes")
	}
}

func TestHMACVerify_DifferentPathRejected(t *testing.T) {
	secret := []byte("k")
	sig := ComputeSignature(secret, "k1", "GET", "/a", "100", "n", nil)
	if err := VerifySignature(secret, "k1", "GET", "/b", "100", "n", nil, sig); err == nil {
		t.Fatal("expected mismatch error when path changes")
	}
}

func TestHMACVerify_DifferentBodyRejected(t *testing.T) {
	secret := []byte("k")
	sig := ComputeSignature(secret, "k1", "POST", "/x", "100", "n", []byte(`{"a":1}`))
	if err := VerifySignature(secret, "k1", "POST", "/x", "100", "n", []byte(`{"a":2}`), sig); err == nil {
		t.Fatal("expected mismatch error when body changes")
	}
}

func TestHMACVerify_DifferentSecretRejected(t *testing.T) {
	sig := ComputeSignature([]byte("a"), "k1", "GET", "/x", "100", "n", nil)
	if err := VerifySignature([]byte("b"), "k1", "GET", "/x", "100", "n", nil, sig); err == nil {
		t.Fatal("expected mismatch when secret changes")
	}
}

func TestHMACVerify_DifferentKeyIDRejected(t *testing.T) {
	secret := []byte("k")
	sig := ComputeSignature(secret, "k1", "GET", "/x", "100", "n", nil)
	if err := VerifySignature(secret, "k2", "GET", "/x", "100", "n", nil, sig); err == nil {
		t.Fatal("expected mismatch when key id is bound into payload but differs")
	}
}

// ---------- Timestamp window cases (boundary cases too) ----------

func TestCheckTimestamp_RejectsTooOld(t *testing.T) {
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if err := CheckTimestamp(old, time.Now(), 5*time.Minute); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Fatalf("expected ErrTimestampOutOfRange, got %v", err)
	}
}

func TestCheckTimestamp_RejectsTooFuture(t *testing.T) {
	future := strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)
	if err := CheckTimestamp(future, time.Now(), 5*time.Minute); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Fatalf("expected ErrTimestampOutOfRange, got %v", err)
	}
}

func TestCheckTimestamp_BoundaryAccepted(t *testing.T) {
	now := time.Now()
	// window is 5 minutes; an exactly -5min and +5min should be accepted (inclusive)
	for _, offset := range []time.Duration{-5 * time.Minute, 5 * time.Minute} {
		ts := now.Add(offset)
		tsStr := strconv.FormatInt(ts.Unix(), 10)
		if err := CheckTimestamp(tsStr, now, 5*time.Minute); err != nil {
			t.Fatalf("expected accept at boundary %v, got %v", offset, err)
		}
	}
}

func TestCheckTimestamp_JustOutsideRejected(t *testing.T) {
	now := time.Now()
	for _, ts := range []time.Time{now.Add(-(5*time.Minute + time.Second)), now.Add(5*time.Minute + time.Second)} {
		tsStr := strconv.FormatInt(ts.Unix(), 10)
		if err := CheckTimestamp(tsStr, now, 5*time.Minute); !errors.Is(err, ErrTimestampOutOfRange) {
			t.Fatalf("expected reject at just-outside boundary %v, got %v", ts, err)
		}
	}
}

func TestCheckTimestamp_NonNumericRejected(t *testing.T) {
	if err := CheckTimestamp("not-a-number", time.Now(), 5*time.Minute); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Fatalf("expected ErrTimestampOutOfRange, got %v", err)
	}
}

func TestCheckTimestamp_DefaultWindow(t *testing.T) {
	if DefaultHMACWindow != 5*time.Minute {
		t.Fatalf("DefaultHMACWindow must be 5 minutes (spec ±300s), got %v", DefaultHMACWindow)
	}
}

// ---------- Nonce replay store cases ----------

func TestNonceStore_FirstWinsSecondRejected(t *testing.T) {
	store := NewMemoryNonceStore(10 * time.Second)
	if err := store.CheckAndSet("k1", "n1"); err != nil {
		t.Fatalf("first call must succeed, got %v", err)
	}
	if err := store.CheckAndSet("k1", "n1"); !errors.Is(err, ErrNonceReplay) {
		t.Fatalf("second call must be ErrNonceReplay, got %v", err)
	}
}

func TestNonceStore_DistinctKeyIDsAllowed(t *testing.T) {
	store := NewMemoryNonceStore(10 * time.Second)
	if err := store.CheckAndSet("k1", "n"); err != nil {
		t.Fatalf("k1/n: %v", err)
	}
	if err := store.CheckAndSet("k2", "n"); err != nil {
		t.Fatalf("k2/n should not collide with k1/n: %v", err)
	}
}

func TestNonceStore_DistinctNoncesAllowed(t *testing.T) {
	store := NewMemoryNonceStore(10 * time.Second)
	if err := store.CheckAndSet("k1", "n1"); err != nil {
		t.Fatalf("k1/n1: %v", err)
	}
	if err := store.CheckAndSet("k1", "n2"); err != nil {
		t.Fatalf("k1/n2 should be allowed: %v", err)
	}
}

func TestNonceStore_TTLExpiry(t *testing.T) {
	store := NewMemoryNonceStore(50 * time.Millisecond)
	if err := store.CheckAndSet("k", "n"); err != nil {
		t.Fatalf("first: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if err := store.CheckAndSet("k", "n"); err != nil {
		t.Fatalf("after TTL expiry, replay should succeed again, got %v", err)
	}
}

// ---------- Authorization header parsing ----------

func TestParseHMACHeader_Valid(t *testing.T) {
	auth := "HMAC key-1,abc,1760000000,nonce-1"
	h, err := ParseHMACHeader(auth)
	if err != nil {
		t.Fatalf("ParseHMACHeader: %v", err)
	}
	if h.KeyID != "key-1" || h.Signature != "abc" || h.Timestamp != "1760000000" || h.Nonce != "nonce-1" {
		t.Fatalf("parsed mismatch: %+v", h)
	}
}

func TestParseHMACHeader_WrongScheme(t *testing.T) {
	if _, err := ParseHMACHeader("Bearer abc"); err == nil {
		t.Fatal("expected scheme mismatch error")
	}
}

func TestParseHMACHeader_MissingFields(t *testing.T) {
	for _, bad := range []string{
		"HMAC",
		"HMAC key-1",
		"HMAC key-1,abc",
		"HMAC key-1,abc,1760000000",
		"HMAC ,a,b,c",
		"HMAC a,,c,d",
		"HMAC a,b,,",
	} {
		if _, err := ParseHMACHeader(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestParseHMACHeader_AllowsVariadicWhitespace(t *testing.T) {
	// Should tolerate surrounding whitespace on each comma-separated field.
	if _, err := ParseHMACHeader("HMAC  key-1 , abc , 1760000000 , nonce-1 "); err != nil {
		t.Fatalf("whitespace-tolerant parse failed: %v", err)
	}
}

// ---------- VerifyRequest: end-to-end integration with a Gin request ----------

func TestVerifyRequest_AcceptsValidSignature(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n-1"
	body := []byte(`{"x":1}`)
	sig := ComputeSignature(secret, "k1", "POST", "/api/v1/x", ts, nonce, body)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/v1/x", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "HMAC k1,"+sig+","+ts+","+nonce)
	req.ContentLength = int64(len(body))
	c.Request = req

	store := NewMemoryNonceStore(time.Minute)
	if err := VerifyRequest(c, HMACVerifyConfig{
		Secret:    secret,
		Now:       now,
		Window:    5 * time.Minute,
		NonceStore: store,
	}); err != nil {
		t.Fatalf("VerifyRequest: %v", err)
	}
}

func TestVerifyRequest_RejectsBadSig(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n"

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/v1/x", strings.NewReader("{}"))
	req.Header.Set("Authorization", "HMAC k1,0000000000000000000000000000000000000000000000000000000000000000,"+ts+","+nonce)
	c.Request = req

	store := NewMemoryNonceStore(time.Minute)
	if err := VerifyRequest(c, HMACVerifyConfig{
		Secret:    secret,
		Now:       now,
		Window:    5 * time.Minute,
		NonceStore: store,
	}); err == nil {
		t.Fatal("expected bad-sig rejection")
	}
}

func TestVerifyRequest_ReplaysNonceOnSecondCall(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "replay-1"
	body := []byte(`{"x":1}`)
	sig := ComputeSignature(secret, "k1", "POST", "/api/v1/x", ts, nonce, body)

	store := NewMemoryNonceStore(time.Minute)
	for i := 0; i < 2; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest("POST", "/api/v1/x", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "HMAC k1,"+sig+","+ts+","+nonce)
		req.ContentLength = int64(len(body))
		c.Request = req
		err := VerifyRequest(c, HMACVerifyConfig{Secret: secret, Now: now, Window: 5 * time.Minute, NonceStore: store})
		if i == 0 && err != nil {
			t.Fatalf("first call should pass: %v", err)
		}
		if i == 1 {
			if !errors.Is(err, ErrNonceReplay) {
				t.Fatalf("second call must be ErrNonceReplay, got %v", err)
			}
		}
	}
}

func TestVerifyRequest_RejectsOutOfWindow(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	oldTs := strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10)
	nonce := "n"
	body := []byte("{}")
	sig := ComputeSignature(secret, "k1", "POST", "/api/v1/x", oldTs, nonce, body)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/v1/x", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "HMAC k1,"+sig+","+oldTs+","+nonce)
	req.ContentLength = int64(len(body))
	c.Request = req

	if err := VerifyRequest(c, HMACVerifyConfig{
		Secret:     secret,
		Now:        now,
		Window:     5 * time.Minute,
		NonceStore: NewMemoryNonceStore(time.Minute),
	}); !errors.Is(err, ErrTimestampOutOfRange) {
		t.Fatalf("expected ErrTimestampOutOfRange, got %v", err)
	}
}

// TestComputeSignature_IsConstantTimeComparable demonstrates we use crypto/hmac
// (which is constant-time). This is a smoke test, not a security proof.
func TestComputeSignature_IsConstantTimeComparable(t *testing.T) {
	a := ComputeSignature([]byte("k"), "id", "GET", "/x", "1", "n", []byte("body"))
	b := ComputeSignature([]byte("k"), "id", "GET", "/x", "1", "n", []byte("body"))
	if !hmac.Equal([]byte(a), []byte(b)) {
		t.Fatal("hmac.Equal should accept identical signatures")
	}
	// make sure shape matches hex of a known digest — the keyID is mixed
	// into the digest, so the expected digest uses the same key + keyID
	// binding as the implementation.
	mac := hmac.New(sha256.New, []byte("k"))
	mac.Write([]byte("id"))
	mac.Write([]byte{0})
	mac.Write([]byte(SignString("GET", "/x", "1", "n", "body")))
	want := hex.EncodeToString(mac.Sum(nil))
	if a != want {
		t.Fatalf("signature should match sha256-hex hmac of canonical string; got=%s want=%s", a, want)
	}
}

// Helper: silence unused warnings if a refactor drops some imports.
var _ = http.MethodGet

// TestMemoryNonceStore_ConcurrentReplayGuard pins the regression that
// motivated the mutex: without it, two goroutines presenting the same
// (keyID, nonce) can both observe an empty slot and double-succeed,
// which is the replay attack CheckAndSet exists to block. With the
// mutex, exactly one of 100 concurrent goroutines wins.
func TestMemoryNonceStore_ConcurrentReplayGuard(t *testing.T) {
	t.Parallel()
	store := NewMemoryNonceStore(time.Minute)
	const goroutines = 100
	var (
		wg       sync.WaitGroup
		okCount  int32
		replays  int32
		other    int32
	)
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			switch err := store.CheckAndSet("k", "n"); {
			case err == nil:
				atomic.AddInt32(&okCount, 1)
			case errors.Is(err, ErrNonceReplay):
				atomic.AddInt32(&replays, 1)
			default:
				atomic.AddInt32(&other, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("expected exactly 1 success under concurrent SETNX, got ok=%d replay=%d other=%d", okCount, replays, other)
	}
	if replays != goroutines-1 {
		t.Fatalf("expected %d replays, got replay=%d other=%d", goroutines-1, replays, other)
	}
	if other != 0 {
		t.Fatalf("unexpected non-replay errors: %d", other)
	}
}
