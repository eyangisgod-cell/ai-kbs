package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ---------- helpers ----------

type stubPrincipal struct {
	hmacUser  *types.User
	jwtUser   *types.User
	apiUser   *types.User
	jwtClaims *JWTClaims
	apiRec    APIKeyRecord
}

func (s *stubPrincipal) UserFromHMAC(_ context.Context, keyID string) (*types.User, error) {
	if keyID == "" {
		return nil, ErrPrincipalNotFound
	}
	if s.hmacUser == nil {
		return nil, ErrPrincipalNotFound
	}
	return s.hmacUser, nil
}

func (s *stubPrincipal) UserFromJWT(_ context.Context, claims *JWTClaims) (*types.User, error) {
	if s.jwtClaims != nil && claims != nil && claims.UserID == s.jwtClaims.UserID {
		return s.jwtUser, nil
	}
	if s.jwtUser != nil {
		return s.jwtUser, nil
	}
	return nil, ErrPrincipalNotFound
}

func (s *stubPrincipal) UserFromAPIKey(_ context.Context, rec APIKeyRecord) (*types.User, error) {
	if rec.KeyID == "" {
		return nil, ErrPrincipalNotFound
	}
	if s.apiUser == nil {
		return nil, ErrPrincipalNotFound
	}
	return s.apiUser, nil
}

func runRequest(mw gin.HandlerFunc, method, path, hdr string, body []byte) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw)
	r.Any("/api/ai-kbs/*any", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if hdr != "" {
		req.Header.Set("Authorization", hdr)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func assertEnvelope(t *testing.T, w *httptest.ResponseRecorder, wantCode envelope.Code, wantStatus int) *envelope.Envelope {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, wantStatus, w.Body.String())
	}
	var env envelope.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, w.Body.String())
	}
	if env.Error == nil || env.Error.Code != wantCode {
		t.Fatalf("env error code = %v, want %s (body=%s)", env.Error, wantCode, w.Body.String())
	}
	return &env
}

// ---------- HMAC middleware (auto) ----------

func TestMiddleware_HMAC_AcceptsValid(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n-hmac-1"
	body := []byte(`{"x":1}`)
	sig := ComputeSignature(secret, "k1", "POST", "/api/ai-kbs/test", ts, nonce, body)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce

	stub := &stubPrincipal{hmacUser: &types.User{ID: "u-hmac", Username: "alice"}}
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  stub,
	}), "POST", "/api/ai-kbs/test", hdr, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestMiddleware_HMAC_RejectsStaleTimestamp(t *testing.T) {
	secret := []byte("shared")
	old := time.Now().Add(-10 * time.Minute)
	ts := strconv.FormatInt(old.Unix(), 10)
	nonce := "n"
	sig := ComputeSignature(secret, "k1", "GET", "/api/ai-kbs/x", ts, nonce, nil)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce

	stub := &stubPrincipal{hmacUser: &types.User{ID: "u"}}
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  stub,
	}), "GET", "/api/ai-kbs/x", hdr, nil)
	assertEnvelope(t, w, envelope.CodeAuthTokenExpired, http.StatusUnauthorized)
}

func TestMiddleware_HMAC_RejectsBadSig(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n"
	hdr := "HMAC k1,0000000000000000000000000000000000000000000000000000000000000000," + ts + "," + nonce

	stub := &stubPrincipal{hmacUser: &types.User{ID: "u"}}
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  stub,
	}), "POST", "/api/ai-kbs/x", hdr, nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusForbidden)
}

func TestMiddleware_HMAC_RejectsReplay(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n-replay"
	body := []byte(`{"x":1}`)
	sig := ComputeSignature(secret, "k1", "POST", "/api/ai-kbs/test", ts, nonce, body)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce

	stub := &stubPrincipal{hmacUser: &types.User{ID: "u"}}
	mw := Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  stub,
	})
	// first call OK
	w1 := runRequest(mw, "POST", "/api/ai-kbs/test", hdr, body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first call should pass, got %d (body=%s)", w1.Code, w1.Body.String())
	}
	// second call replay
	w2 := runRequest(mw, "POST", "/api/ai-kbs/test", hdr, body)
	assertEnvelope(t, w2, envelope.CodeAuthTokenExpired, http.StatusUnauthorized)
}

func TestMiddleware_HMAC_RejectsMissingHeader(t *testing.T) {
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: []byte("k"),
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{},
	}), "GET", "/api/ai-kbs/x", "", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_HMAC_MalformedHeader(t *testing.T) {
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: []byte("k"),
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{},
	}), "GET", "/api/ai-kbs/x", "HMAC broken-shape", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_HMAC_UnknownKeyID(t *testing.T) {
	secret := []byte("shared")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n"
	sig := ComputeSignature(secret, "ghost", "GET", "/api/ai-kbs/x", ts, nonce, nil)
	hdr := "HMAC ghost," + sig + "," + ts + "," + nonce

	// stub returns ErrPrincipalNotFound when keyID is "ghost"
	stub := &stubPrincipal{hmacUser: nil}
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  stub,
	}), "GET", "/api/ai-kbs/x", hdr, nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

// ---------- JWT middleware ----------

func TestMiddleware_JWT_AcceptsValid(t *testing.T) {
	signer := NewHS256([]byte("k"))
	tok, _ := signer.Sign(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})

	stub := &stubPrincipal{jwtUser: &types.User{ID: "u1", Username: "alice"}}
	w := runRequest(Middleware(MiddlewareConfig{
		JWTSigner: signer,
		Principal: stub,
	}), "GET", "/api/ai-kbs/x", "Bearer "+tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestMiddleware_JWT_RejectsExpired(t *testing.T) {
	signer := NewHS256([]byte("k"))
	tok, _ := signer.Sign(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()})

	stub := &stubPrincipal{jwtUser: &types.User{ID: "u1"}}
	w := runRequest(Middleware(MiddlewareConfig{
		JWTSigner: signer,
		Principal: stub,
	}), "GET", "/api/ai-kbs/x", "Bearer "+tok, nil)
	assertEnvelope(t, w, envelope.CodeAuthTokenExpired, http.StatusUnauthorized)
}

func TestMiddleware_JWT_RejectsInvalid(t *testing.T) {
	signer := NewHS256([]byte("k"))
	stub := &stubPrincipal{jwtUser: &types.User{ID: "u1"}}
	w := runRequest(Middleware(MiddlewareConfig{
		JWTSigner: signer,
		Principal: stub,
	}), "GET", "/api/ai-kbs/x", "Bearer not-a-jwt", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_JWT_RejectsMissingBearerPrefix(t *testing.T) {
	signer := NewHS256([]byte("k"))
	stub := &stubPrincipal{jwtUser: &types.User{ID: "u1"}}
	w := runRequest(Middleware(MiddlewareConfig{
		JWTSigner: signer,
		Principal: stub,
	}), "GET", "/api/ai-kbs/x", "Basic dXNlcjpwYXNz", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_JWT_NoSignerConfigured(t *testing.T) {
	w := runRequest(Middleware(MiddlewareConfig{
		Principal: &stubPrincipal{},
	}), "GET", "/api/ai-kbs/x", "Bearer whatever", nil)
	assertEnvelope(t, w, envelope.CodeInternalError, http.StatusInternalServerError)
}

// ---------- API Key middleware ----------

func TestMiddleware_APIKey_AcceptsValid(t *testing.T) {
	hash, _ := HashAPIKeySecret("secret-1")
	store := staticRecordStore{
		"ak_abc": {
			KeyID:        "ak_abc",
			Hash:         hash,
			Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve},
		},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u", Username: "alice"}}
	raw := "ak_abc.secret-1"
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey "+raw, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestMiddleware_APIKey_RejectsRevoked(t *testing.T) {
	hash, _ := HashAPIKeySecret("secret-1")
	past := time.Now().Add(-time.Hour)
	store := staticRecordStore{
		"ak_abc": {KeyID: "ak_abc", Hash: hash, RevokedAt: &past},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u"}}
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey ak_abc.secret-1", nil)
	assertEnvelope(t, w, envelope.CodeAuthAPIKeyRevoked, http.StatusUnauthorized)
}

func TestMiddleware_APIKey_RejectsExpired(t *testing.T) {
	hash, _ := HashAPIKeySecret("secret-1")
	past := time.Now().Add(-time.Hour)
	store := staticRecordStore{
		"ak_abc": {KeyID: "ak_abc", Hash: hash, ExpiresAt: &past},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u"}}
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey ak_abc.secret-1", nil)
	assertEnvelope(t, w, envelope.CodeAuthAPIKeyRevoked, http.StatusUnauthorized)
}

func TestMiddleware_APIKey_RejectsUnknown(t *testing.T) {
	store := staticRecordStore{}
	stub := &stubPrincipal{}
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey ak_ghost.something", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_APIKey_RejectsMalformed(t *testing.T) {
	store := staticRecordStore{}
	stub := &stubPrincipal{}
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey not-a-key", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_APIKey_BadSecret(t *testing.T) {
	hash, _ := HashAPIKeySecret("real-secret")
	store := staticRecordStore{
		"ak_abc": {KeyID: "ak_abc", Hash: hash, Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u"}}
	w := runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}), "GET", "/api/ai-kbs/x", "APIKey ak_abc.wrong-secret", nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

// TestMiddleware_APIKey_ErrorMessagesCollapsed pins the regression that
// motivated the unification: an unknown key, a malformed key, and a
// wrong-secret must all return the SAME user-facing message so an
// attacker cannot probe whether a key ID exists by comparing the wire
// text. Internal observability keeps distinct envelope codes (tested
// above) — only the message text is collapsed.
func TestMiddleware_APIKey_ErrorMessagesCollapsed(t *testing.T) {
	hash, _ := HashAPIKeySecret("real-secret")
	store := staticRecordStore{
		"ak_abc": {KeyID: "ak_abc", Hash: hash, Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u"}}
	cases := []struct {
		name, hdr string
	}{
		{"unknown", "APIKey ak_ghost.something"},
		{"malformed", "APIKey not-a-key"},
		{"bad-secret", "APIKey ak_abc.wrong-secret"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := runRequest(Middleware(MiddlewareConfig{
				APIKeyLookup: store,
				Principal:    stub,
			}), "GET", "/api/ai-kbs/x", tc.hdr, nil)
			env := assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
			if env.Error == nil || env.Error.Message != apiKeyVerifyFailedMessage {
				t.Fatalf("error.message = %q, want %q", env.Error.Message, apiKeyVerifyFailedMessage)
			}
		})
	}
}

func TestMiddleware_APIKey_InjectsCapabilities(t *testing.T) {
	hash, _ := HashAPIKeySecret("s")
	store := staticRecordStore{
		"ak_abc": {
			KeyID:        "ak_abc",
			Hash:         hash,
			Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve, types.APIKeyCapabilityIngest},
		},
	}
	stub := &stubPrincipal{apiUser: &types.User{ID: "u"}}

	// Custom route so we can inspect the capabilities injected.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	seenCaps := []string{}
	r.Use(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    stub,
	}))
	r.GET("/api/ai-kbs/x", func(c *gin.Context) {
		v, _ := c.Get("api_key_capabilities")
		if arr, ok := v.([]string); ok {
			seenCaps = arr
		}
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/api/ai-kbs/x", nil)
	req.Header.Set("Authorization", "APIKey ak_abc.s")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(seenCaps) != 2 {
		t.Fatalf("expected 2 capabilities, got %v", seenCaps)
	}
	want := map[string]bool{"retrieve": true, "ingest": true}
	for _, c := range seenCaps {
		if !want[c] {
			t.Fatalf("unexpected cap %q", c)
		}
	}
}

// ---------- Auto mode picks by prefix ----------

func TestMiddleware_AutoRoutesByPrefix(t *testing.T) {
	// HMAC
	secret := []byte("k")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n"
	body := []byte(`{"a":1}`)
	sig := ComputeSignature(secret, "k1", "POST", "/api/ai-kbs/test", ts, nonce, body)
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{hmacUser: &types.User{ID: "u"}},
	}), "POST", "/api/ai-kbs/test", "HMAC k1,"+sig+","+ts+","+nonce, body)
	if w.Code != http.StatusOK {
		t.Fatalf("HMAC path should succeed in auto mode, got %d (body=%s)", w.Code, w.Body.String())
	}
	// APIKey
	hash, _ := HashAPIKeySecret("s")
	store := staticRecordStore{
		"ak_x": {KeyID: "ak_x", Hash: hash, Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}},
	}
	w = runRequest(Middleware(MiddlewareConfig{
		APIKeyLookup: store,
		Principal:    &stubPrincipal{apiUser: &types.User{ID: "u"}},
	}), "GET", "/api/ai-kbs/test", "APIKey ak_x.s", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("APIKey path should succeed in auto mode, got %d", w.Code)
	}
	// JWT
	signer := NewHS256([]byte("k"))
	tok, _ := signer.Sign(JWTClaims{UserID: "u", ExpiresAt: now.Add(time.Hour).Unix()})
	w = runRequest(Middleware(MiddlewareConfig{
		JWTSigner: signer,
		Principal: &stubPrincipal{jwtUser: &types.User{ID: "u"}},
	}), "GET", "/api/ai-kbs/test", "Bearer "+tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("JWT path should succeed in auto mode, got %d", w.Code)
	}
}

// ---------- Redis-backed nonce store ----------

// TestNonceStore_Redis is the integration test for the Redis SETNX
// implementation. We use miniredis so the test is hermetic.
func TestNonceStore_Redis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisNonceStore(rdb, DefaultNonceTTL)

	if err := store.CheckAndSet("k1", "n1"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := store.CheckAndSet("k1", "n1"); err == nil || err != ErrNonceReplay {
		t.Fatalf("expected ErrNonceReplay on second call, got %v", err)
	}
	if err := store.CheckAndSet("k1", "n2"); err != nil {
		t.Fatalf("different nonce should be allowed: %v", err)
	}
	if err := store.CheckAndSet("k2", "n1"); err != nil {
		t.Fatalf("different keyID should be allowed: %v", err)
	}
	// Force expiry and retry
	mr.FastForward(DefaultNonceTTL + time.Second)
	if err := store.CheckAndSet("k1", "n1"); err != nil {
		t.Fatalf("after TTL expiry, replay should succeed again: %v", err)
	}
}

// TestNonceStore_Redis_DistinguishesConcurrentCalls ensures the SETNX
// is atomic: the second of two parallel callers must see ErrNonceReplay.
func TestNonceStore_Redis_Concurrent(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisNonceStore(rdb, DefaultNonceTTL)

	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- store.CheckAndSet("kk", "nn")
		}()
	}
	successes, replays := 0, 0
	for i := 0; i < 10; i++ {
		e := <-done
		switch {
		case e == nil:
			successes++
		case e == ErrNonceReplay:
			replays++
		default:
			t.Fatalf("unexpected error: %v", e)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly 1 success across 10 concurrent SETNX calls, got %d", successes)
	}
	if replays != 9 {
		t.Fatalf("expected 9 replays, got %d", replays)
	}
}

// Suppress unused imports when miniredis is conditionally included.
var _ = strings.TrimSpace

// ---------- Inject helper tests ----------

func TestInject_AttachesUserAndCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/ai-kbs/x", nil)
	user := &types.User{ID: "u1"}
	Inject(c, user, []string{"retrieve", "ingest"})

	if v, ok := c.Get("user"); !ok || v.(*types.User).ID != "u1" {
		t.Fatalf("user not attached to keys: %v ok=%v", v, ok)
	}
	if v, ok := c.Get(types.UserContextKey.String()); !ok || v.(*types.User).ID != "u1" {
		t.Fatalf("user not attached to user-context-key: %v ok=%v", v, ok)
	}
	if v, ok := c.Get("api_key_capabilities"); !ok {
		t.Fatalf("capabilities not attached")
	} else if got := v.([]string); len(got) != 2 {
		t.Fatalf("capabilities slice len = %d, want 2", len(got))
	}
}

func TestInject_NilUserSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/ai-kbs/x", nil)
	Inject(c, nil, nil)
	if _, ok := c.Get("user"); ok {
		t.Fatal("nil user should not be attached")
	}
}

// ---------- time / window override tests ----------

func TestMiddleware_HMAC_HonorsWindowOverride(t *testing.T) {
	// Build a signature with a timestamp 4 minutes ago; with the
	// default 5-min window it would pass, but a 1-min override must
	// reject it.
	secret := []byte("k")
	now := time.Now()
	ts := strconv.FormatInt(now.Add(-4*time.Minute).Unix(), 10)
	nonce := "n"
	sig := ComputeSignature(secret, "k1", "GET", "/api/ai-kbs/x", ts, nonce, nil)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce

	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		HMACWindow: 1 * time.Minute, // tighten
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{hmacUser: &types.User{ID: "u"}},
	}), "GET", "/api/ai-kbs/x", hdr, nil)
	assertEnvelope(t, w, envelope.CodeAuthTokenExpired, http.StatusUnauthorized)
}

func TestMiddleware_HMAC_DefaultWindowAppliedWhenZero(t *testing.T) {
	// Window of zero must fall back to DefaultHMACWindow (5 min), so
	// a 2-minute-old signature is accepted.
	secret := []byte("k")
	now := time.Now()
	ts := strconv.FormatInt(now.Add(-2*time.Minute).Unix(), 10)
	nonce := "n"
	sig := ComputeSignature(secret, "k1", "GET", "/api/ai-kbs/x", ts, nonce, nil)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce

	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		// HMACWindow: 0
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{hmacUser: &types.User{ID: "u"}},
	}), "GET", "/api/ai-kbs/x", hdr, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with default 5-min window, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// ---------- pure-forced mode tests ----------

func TestMiddleware_ForcedJWTModeIgnoresHMACHeader(t *testing.T) {
	secret := []byte("k")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := "n"
	sig := ComputeSignature(secret, "k1", "GET", "/api/ai-kbs/x", ts, nonce, nil)
	hdr := "HMAC k1," + sig + "," + ts + "," + nonce // looks like HMAC...

	w := runRequest(Middleware(MiddlewareConfig{
		Mode:      AuthModeJWT,
		JWTSigner: NewHS256([]byte("k")),
		Principal: &stubPrincipal{jwtUser: &types.User{ID: "u"}},
	}), "GET", "/api/ai-kbs/x", hdr, nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestMiddleware_ForcedAPIKeyModeIgnoresBearerHeader(t *testing.T) {
	signer := NewHS256([]byte("k"))
	tok, _ := signer.Sign(JWTClaims{UserID: "u", ExpiresAt: now().Add(time.Hour).Unix()})

	w := runRequest(Middleware(MiddlewareConfig{
		Mode:        AuthModeAPIKey,
		APIKeyLookup: staticRecordStore{},
		Principal:    &stubPrincipal{},
	}), "GET", "/api/ai-kbs/x", "Bearer "+tok, nil)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

// ---------- body-too-large guard ----------

func TestMiddleware_HMAC_RejectsOversizedBody(t *testing.T) {
	secret := []byte("k")
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	// Compose an Authorization header but with a body that exceeds the
	// configured 1 KiB cap. The size check fires after timestamp
	// validation but before signature validation.
	w := runRequest(Middleware(MiddlewareConfig{
		HMACSecret: secret,
		MaxBody:    1024, // 1 KiB
		NonceStore: NewMemoryNonceStore(time.Minute),
		Principal:  &stubPrincipal{hmacUser: &types.User{ID: "u"}},
	}), "POST", "/api/ai-kbs/x", "HMAC k1,deadbeef,"+ts+",n", bytes.Repeat([]byte("a"), 2048))
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusBadRequest)
}

// now returns a non-monotonic-but-deterministic clock for the JWT
// forced-mode tests; we want time.Hour offsets to compute against the
// process boot time, not a free-floating zero.
func now() time.Time { return time.Now() }

