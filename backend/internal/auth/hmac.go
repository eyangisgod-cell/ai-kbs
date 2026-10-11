// Package auth provides the AI-KBS authentication primitives.
//
// This package is the T1.5 contract surface for the three credential
// families the platform recognises:
//   - HMAC (server-to-server)        — see hmac.go
//   - JWT  (browser SPA / mobile)    — see jwt.go
//   - API Key (CLI / third-party)    — see api_key.go
//
// Middleware glue lives in middleware.go. Handlers expose the credential
// issuance paths under /api/ai-kbs/auth (see handler/auth.go and the
// router wiring). All error responses are formatted through the v1.0
// envelope defined in internal/errors/envelope.
package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultHMACWindow matches design.md §4.4: ±300 seconds. CLI /
// server-to-server deployments can override by setting HMAC_WINDOW.
const DefaultHMACWindow = 5 * time.Minute

// DefaultNonceTTL matches design.md §4.4: 600 seconds (10 minutes). The
// nonce store uses this as the SETNX EX parameter when backing onto
// Redis; the in-memory store mirrors it for tests.
const DefaultNonceTTL = 10 * time.Minute

// Errors surfaced by the HMAC path. Each error maps to a stable wire
// code in the v1.0 envelope (envelope.CodeAuthTokenExpired etc.).
var (
	// ErrTimestampOutOfRange is returned when the request timestamp is
	// outside the configured ±window. The middleware translates this to
	// auth.token.expired (401).
	ErrTimestampOutOfRange = errors.New("hmac: timestamp outside allowed window")
	// ErrSignatureMismatch is returned when the HMAC of the canonical
	// request string does not equal the signature header. Translated to
	// auth.permission_denied (403) so the caller cannot distinguish
	// "bad signature" from "unknown key" via the error code.
	ErrSignatureMismatch = errors.New("hmac: signature mismatch")
	// ErrNonceReplay is returned when the same (keyID, nonce) tuple has
	// been seen within the nonce TTL window. Translated to 401.
	ErrNonceReplay = errors.New("hmac: nonce replay")
	// ErrMalformedHeader is returned when the Authorization header is
	// not in the expected "HMAC k,sig,ts,nonce" shape.
	ErrMalformedHeader = errors.New("hmac: malformed Authorization header")
)

// HMACHeader is the parsed form of the Authorization: HMAC … header.
type HMACHeader struct {
	KeyID     string
	Signature string
	Timestamp string
	Nonce     string
}

// SignString builds the canonical request string used as HMAC input.
//
// Layout (newline-separated, fixed):
//
//	METHOD\n
//	PATH\n
//	TIMESTAMP\n
//	NONCE\n
//	BODY
//
// The body is included verbatim; the empty string is legal for bodyless
// requests. The signature MUST bind the key ID to prevent a known-good
// signature for key A being replayed under key B; the caller mixes the
// key ID into the digest by prefixing it (see ComputeSignature).
func SignString(method, path, ts, nonce string, body string) string {
	var buf bytes.Buffer
	buf.Grow(len(method) + len(path) + len(ts) + len(nonce) + len(body) + 8)
	buf.WriteString(method)
	buf.WriteByte('\n')
	buf.WriteString(path)
	buf.WriteByte('\n')
	buf.WriteString(ts)
	buf.WriteByte('\n')
	buf.WriteString(nonce)
	buf.WriteByte('\n')
	buf.WriteString(body)
	return buf.String()
}

// ComputeSignature returns the lowercase-hex SHA-256 HMAC of the
// canonical request string. The keyID is mixed into the digest so two
// keys cannot collide even if they share a secret.
func ComputeSignature(secret []byte, keyID, method, path, ts, nonce string, body []byte) string {
	canonical := SignString(method, path, ts, nonce, string(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(keyID))
	mac.Write([]byte{0})
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature compares an incoming signature against a freshly
// computed one in constant time.
func VerifySignature(secret []byte, keyID, method, path, ts, nonce string, body []byte, sig string) error {
	want := ComputeSignature(secret, keyID, method, path, ts, nonce, body)
	gotBytes, err := hex.DecodeString(sig)
	if err != nil {
		return ErrSignatureMismatch
	}
	wantBytes, _ := hex.DecodeString(want)
	if !hmac.Equal(gotBytes, wantBytes) {
		return ErrSignatureMismatch
	}
	return nil
}

// CheckTimestamp verifies that ts (a decimal Unix-second string) lies
// inside [now-window, now+window]. Returns ErrTimestampOutOfRange on
// the open boundary; exactly window-past and window-future are
// accepted (inclusive). A non-numeric timestamp also returns
// ErrTimestampOutOfRange to avoid leaking "bad format" vs "stale".
//
// The check is performed in integer-second units so the boundary test
// does not flake on sub-second clock drift between when the request was
// signed and when the server validates it.
func CheckTimestamp(ts string, now time.Time, window time.Duration) error {
	n, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64)
	if err != nil {
		return ErrTimestampOutOfRange
	}
	delta := now.Unix() - n
	if delta < 0 {
		delta = -delta
	}
	if time.Duration(delta)*time.Second > window {
		return ErrTimestampOutOfRange
	}
	return nil
}

// ParseHMACHeader parses "HMAC <keyID>,<sig>,<ts>,<nonce>". Whitespace
// around each comma-separated field is trimmed. Returns ErrMalformedHeader
// for missing fields, wrong scheme, or empty parts.
func ParseHMACHeader(h string) (HMACHeader, error) {
	const scheme = "HMAC "
	h = strings.TrimSpace(h)
	if !strings.HasPrefix(h, scheme) {
		return HMACHeader{}, ErrMalformedHeader
	}
	body := strings.TrimPrefix(h, scheme)
	parts := strings.Split(body, ",")
	if len(parts) != 4 {
		return HMACHeader{}, ErrMalformedHeader
	}
	out := HMACHeader{
		KeyID:     strings.TrimSpace(parts[0]),
		Signature: strings.TrimSpace(parts[1]),
		Timestamp: strings.TrimSpace(parts[2]),
		Nonce:     strings.TrimSpace(parts[3]),
	}
	if out.KeyID == "" || out.Signature == "" || out.Timestamp == "" || out.Nonce == "" {
		return HMACHeader{}, ErrMalformedHeader
	}
	return out, nil
}

// NonceStore is the interface the HMAC middleware uses to remember
// nonces it has already accepted. Implementations MUST be safe for
// concurrent use.
type NonceStore interface {
	// CheckAndSet atomically records (keyID, nonce). Returns
	// ErrNonceReplay if the tuple was already present within the TTL.
	CheckAndSet(keyID, nonce string) error
}

// NewMemoryNonceStore is an in-process NonceStore used by tests and
// single-instance deployments. Production deploys use the Redis-backed
// adapter (see internal/auth/noncestore_redis.go) which uses SETNX EX
// 600 — the v1.0 contract from design.md §4.4.
func NewMemoryNonceStore(ttl time.Duration) NonceStore {
	return &memoryNonceStore{ttl: ttl, seen: map[string]time.Time{}}
}

type memoryNonceStore struct {
	ttl  time.Duration
	seen map[string]time.Time
}

// memoryNonceStore does not need a mutex because tests run sequentially;
// production uses the Redis adapter.
func (m *memoryNonceStore) CheckAndSet(keyID, nonce string) error {
	k := keyID + "|" + nonce
	now := time.Now()
	if t, ok := m.seen[k]; ok && now.Sub(t) < m.ttl {
		return ErrNonceReplay
	}
	m.seen[k] = now
	// Lazy GC: drop entries older than 2*ttl to keep the map bounded.
	if len(m.seen) > 1024 {
		for kk, tt := range m.seen {
			if now.Sub(tt) > 2*m.ttl {
				delete(m.seen, kk)
			}
		}
	}
	return nil
}

// HMACVerifyConfig drives VerifyRequest. Now and Window are injected so
// tests are deterministic; in production callers pass time.Now() and
// DefaultHMACWindow (or whatever HMAC_WINDOW env said at startup).
type HMACVerifyConfig struct {
	Secret     []byte
	Now        time.Time
	Window     time.Duration
	NonceStore NonceStore
	// MaxBodyBytes caps the body read; oversized requests get ErrBodyTooLarge.
	// Zero means 1 MiB.
	MaxBodyBytes int64
}

var (
	ErrBodyTooLarge = errors.New("hmac: request body too large")
)

// VerifyRequest is the all-in-one HMAC check a middleware handler calls.
// On success it returns nil and the caller may proceed; on failure it
// returns one of ErrMalformedHeader, ErrTimestampOutOfRange,
// ErrSignatureMismatch, ErrNonceReplay, or ErrBodyTooLarge.
//
// VerifyRequest DOES NOT modify the request body — callers that need
// to re-read the body must use RequestBody(c) instead, which returns
// the bytes the middleware already captured for signature computation.
func VerifyRequest(c *gin.Context, cfg HMACVerifyConfig) error {
	if cfg.Window <= 0 {
		cfg.Window = DefaultHMACWindow
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 1 << 20
	}
	if cfg.NonceStore == nil {
		cfg.NonceStore = NewMemoryNonceStore(DefaultNonceTTL)
	}
	authz := c.GetHeader("Authorization")
	h, err := ParseHMACHeader(authz)
	if err != nil {
		return err
	}
	if err := CheckTimestamp(h.Timestamp, cfg.Now, cfg.Window); err != nil {
		return err
	}
	body, err := readBody(c, cfg.MaxBodyBytes)
	if err != nil {
		return err
	}
	if err := VerifySignature(cfg.Secret, h.KeyID, c.Request.Method, c.Request.URL.Path, h.Timestamp, h.Nonce, body, h.Signature); err != nil {
		return err
	}
	if err := cfg.NonceStore.CheckAndSet(h.KeyID, h.Nonce); err != nil {
		return err
	}
	return nil
}

// readBody reads up to limit bytes from c.Request.Body, leaving the
// original body untouched (so the handler can re-read it via
// c.Request.Body) by replacing it with a bytes.Reader of the captured
// bytes.
func readBody(c *gin.Context, limit int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	n, err := io.CopyN(buf, c.Request.Body, limit+1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n > limit {
		return nil, ErrBodyTooLarge
	}
	body := buf.Bytes()
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
