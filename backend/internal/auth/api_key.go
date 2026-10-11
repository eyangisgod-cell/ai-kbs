package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"golang.org/x/crypto/bcrypt"
)

// APIKeySecretPrefix is the wire-format prefix for every issued key. The
// CLI uses it to recognise its own credential type and reject foreign
// strings before the bcrypt round-trip.
const APIKeySecretPrefix = "ak_"

// API key errors. Each maps to a stable envelope code (see codes.go).
var (
	ErrAPIKeyMalformed          = errors.New("apikey: malformed")
	ErrAPIKeyNotFound           = errors.New("apikey: not found")
	ErrAPIKeySecretMismatch     = errors.New("apikey: secret mismatch")
	ErrAPIKeyExpired            = errors.New("apikey: expired")
	ErrAPIKeyRevoked            = errors.New("apikey: revoked")
	ErrAPIKeyInsufficientCapability = errors.New("apikey: insufficient capability")
)

// NewAPIKey mints a fresh (keyID, secret) pair. The raw string the
// caller sees is "<prefix><keyID>.<secret>" — the keyID is the
// public / loggable part and the secret is the bearer half.
//
// Generation uses crypto/rand for both halves; the secret is base64-url
// encoded so it survives shell quoting and JSON transport without
// escaping. 32 random bytes give 256 bits of entropy, well past the
// bcrypt cost-factor's salt space.
func NewAPIKey() (raw, keyID, secret string) {
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	keyID = APIKeySecretPrefix + base64.RawURLEncoding.EncodeToString(idBytes)

	secBytes := make([]byte, 32)
	_, _ = rand.Read(secBytes)
	secret = base64.RawURLEncoding.EncodeToString(secBytes)

	raw = keyID + "." + secret
	return raw, keyID, secret
}

// ParseAPIKey splits a raw "ak_xxx.yyy" string into (keyID, secret).
// Whitespace around the input is trimmed. Empty halves, missing dot,
// missing prefix, or extra dots all yield ErrAPIKeyMalformed.
func ParseAPIKey(raw string) (keyID, secret string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ErrAPIKeyMalformed
	}
	if !strings.HasPrefix(raw, APIKeySecretPrefix) {
		return "", "", ErrAPIKeyMalformed
	}
	body := strings.TrimPrefix(raw, APIKeySecretPrefix)
	dot := strings.Index(body, ".")
	if dot <= 0 || dot == len(body)-1 {
		return "", "", ErrAPIKeyMalformed
	}
	idPart := body[:dot]
	secPart := body[dot+1:]
	// Reject double-dot or trailing dot shapes (already covered by dot==len-1).
	if strings.Contains(secPart, ".") {
		return "", "", ErrAPIKeyMalformed
	}
	if idPart == "" || secPart == "" {
		return "", "", ErrAPIKeyMalformed
	}
	return APIKeySecretPrefix + idPart, secPart, nil
}

// HashAPIKeySecret bcrypt-hashes a freshly minted secret. The cost is
// the package default (currently 10). Callers persist the hash; the
// plaintext is shown to the user exactly once at creation time and is
// never logged.
func HashAPIKeySecret(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("apikey: empty secret")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("apikey hash: %w", err)
	}
	return string(h), nil
}

// VerifyAPIKeySecret compares a plaintext secret against a stored
// bcrypt hash. Returns nil on match, bcrypt's MismatchedHashAndPassword
// error (wrapped) on mismatch.
func VerifyAPIKeySecret(hash, plain string) error {
	if hash == "" || plain == "" {
		return ErrAPIKeySecretMismatch
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err != nil {
		return ErrAPIKeySecretMismatch
	}
	return nil
}

// APIKeyRecord is the in-memory view of a tenant_api_keys row used by
// the auth package. Repositories translate their own rows into this
// shape; the auth package never touches the database directly.
//
// The fields are the smallest set the contract needs:
//   - KeyID: public identifier (used for logging and nonce store
//     partitioning)
//   - Hash: bcrypt hash of the secret
//   - Capabilities / FullAccess: authorization decision inputs
//   - ExpiresAt / RevokedAt: lifecycle gates
type APIKeyRecord struct {
	KeyID         string
	Hash          string
	FullAccess    bool
	Capabilities  []types.APIKeyCapability
	KnowledgeBaseIDs []string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
}

// IsRevoked reports whether the record has a non-nil RevokedAt in the
// past. Future-dated RevokedAt is a "scheduled revocation" and is
// treated as active for the moment.
func (r APIKeyRecord) IsRevoked(now time.Time) bool {
	if r.RevokedAt == nil {
		return false
	}
	return !r.RevokedAt.After(now)
}

// IsExpired reports whether the record's ExpiresAt is in the past.
// A nil ExpiresAt means "no expiration".
func (r APIKeyRecord) IsExpired(now time.Time) bool {
	if r.ExpiresAt == nil {
		return false
	}
	return !r.ExpiresAt.After(now)
}

// HasCapability reports whether the record grants the supplied
// capability. FullAccess keys grant every capability, including
// platform-level ones, because they are equivalent to a JWT Owner in
// every meaningful way. Empty capability strings are never granted.
//
// Constant-time is not a property here because the caller already
// authenticated via bcrypt; capability check is purely a lookup.
func (r APIKeyRecord) HasCapability(c types.APIKeyCapability) bool {
	if c == "" {
		return false
	}
	if r.FullAccess {
		return true
	}
	want := types.NormalizeAPIKeyCapability(c)
	if want == "" {
		return false
	}
	for _, item := range r.Capabilities {
		if subtle.ConstantTimeCompare([]byte(item), []byte(want)) == 1 {
			return true
		}
	}
	return false
}

// Authorize returns nil when the record permits the capability, or
// ErrAPIKeyInsufficientCapability otherwise. FullAccess keys always pass.
func (r APIKeyRecord) Authorize(c types.APIKeyCapability) error {
	if !r.HasCapability(c) {
		return ErrAPIKeyInsufficientCapability
	}
	return nil
}

// VerifySecret checks the supplied plaintext secret against the
// stored hash AND enforces the lifecycle gates (expiry / revocation).
// The order is deliberate: hash mismatch is leaked via a generic
// "secret mismatch" so attackers cannot probe which key IDs exist.
func (r APIKeyRecord) VerifySecret(plain string) error {
	now := time.Now()
	if r.IsRevoked(now) {
		return ErrAPIKeyRevoked
	}
	if r.IsExpired(now) {
		return ErrAPIKeyExpired
	}
	if err := VerifyAPIKeySecret(r.Hash, plain); err != nil {
		return ErrAPIKeySecretMismatch
	}
	return nil
}

// APIKeyLookup is the dependency the auth package needs to resolve a
// key ID into a record. Repositories satisfy it; tests can pass a map.
type APIKeyLookup interface {
	GetByID(keyID string) (APIKeyRecord, bool)
}

// ResolveAPIKey is the high-level entry the middleware uses. It splits
// the raw "ak_xxx.yyy" string, looks up the record by key ID, and
// verifies the secret plus lifecycle in one call.
func ResolveAPIKey(raw string, store APIKeyLookup) (APIKeyRecord, error) {
	keyID, secret, err := ParseAPIKey(raw)
	if err != nil {
		return APIKeyRecord{}, ErrAPIKeyMalformed
	}
	rec, ok := store.GetByID(keyID)
	if !ok {
		return APIKeyRecord{}, ErrAPIKeyNotFound
	}
	if err := rec.VerifySecret(secret); err != nil {
		return APIKeyRecord{}, err
	}
	return rec, nil
}
