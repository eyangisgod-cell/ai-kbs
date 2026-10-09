package domain

import "time"

// APIKey is the credential row for service-to-service auth. Maps to
// the `api_keys` table. The repository NEVER logs the key_hash —
// logs at debug surface only key_id and a redacted hash prefix.
type APIKey struct {
	ID           string
	UserID       string
	WorkspaceID  string
	KeyID        string
	// KeyHash is the bcrypt digest of the secret half. Callers receive
	// it from GetByKeyID; comparisons are bcrypt's domain, not the
	// repository's.
	KeyHash       string
	Name          string
	Capabilities  []string
	SpaceIDs      []string
	ExpiresAt     *time.Time
	LastUsedAt    *time.Time
	CreatedAt     time.Time
	RevokedAt     *time.Time
}

// IsRevoked reports whether the API key has been revoked.
func (k *APIKey) IsRevoked() bool { return k != nil && k.RevokedAt != nil }