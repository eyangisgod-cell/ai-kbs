package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/types"
)

// ---------- API Key shape & format ----------

func TestAPIKey_FormatParsing(t *testing.T) {
	raw, keyID, secret := NewAPIKey()
	if !strings.HasPrefix(raw, "ak_") {
		t.Fatalf("raw key missing ak_ prefix: %q", raw)
	}
	if keyID == "" {
		t.Fatal("keyID must be non-empty")
	}
	if secret == "" {
		t.Fatal("secret must be non-empty")
	}
	parsedID, parsedSecret, err := ParseAPIKey(raw)
	if err != nil {
		t.Fatalf("ParseAPIKey: %v", err)
	}
	if parsedID != keyID || parsedSecret != secret {
		t.Fatalf("parse mismatch: id=%s secret=%s want id=%s secret=%s",
			parsedID, parsedSecret, keyID, secret)
	}
}

func TestAPIKey_ParseRejectsBadShape(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-key",
		"ak_",
		"ak_only_one_part",
		"ak_xxx..yyy",       // empty secret
		"ak_.yyy",           // empty id
		"ak_xxx.",           // empty secret
		"xx_abc.def",        // wrong prefix
	} {
		if _, _, err := ParseAPIKey(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestAPIKey_ParseToleratesPadding(t *testing.T) {
	raw, id, secret := NewAPIKey()
	withPadding := "  " + raw + "\n"
	pid, psecret, err := ParseAPIKey(withPadding)
	if err != nil {
		t.Fatalf("ParseAPIKey with whitespace: %v", err)
	}
	if pid != id || psecret != secret {
		t.Fatalf("whitespace-stripped parse mismatch")
	}
}

// ---------- bcrypt hashing + verification ----------

func TestAPIKey_HashAndVerify(t *testing.T) {
	hash, err := HashAPIKeySecret("plaintext-secret")
	if err != nil {
		t.Fatalf("HashAPIKeySecret: %v", err)
	}
	if hash == "" || hash == "plaintext-secret" {
		t.Fatalf("hash must not be plaintext, got %q", hash)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("bcrypt hash should start with $2, got %q", hash)
	}
	if err := VerifyAPIKeySecret(hash, "plaintext-secret"); err != nil {
		t.Fatalf("verify same secret: %v", err)
	}
}

func TestAPIKey_VerifyRejectsWrong(t *testing.T) {
	hash, _ := HashAPIKeySecret("right")
	if err := VerifyAPIKeySecret(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

// ---------- APIKeyRecord ----------

func TestAPIKeyRecord_VerifyActive(t *testing.T) {
	hash, _ := HashAPIKeySecret("sec")
	rec := APIKeyRecord{
		KeyID:        "ak_test123",
		Hash:         hash,
		FullAccess:   false,
		Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve, types.APIKeyCapabilityIngest},
		ExpiresAt:    nil,
		RevokedAt:    nil,
	}
	if err := rec.VerifySecret("sec"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := rec.VerifySecret("bad"); err == nil {
		t.Fatal("verify should reject bad secret")
	}
}

func TestAPIKeyRecord_VerifyRejectsExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	rec := APIKeyRecord{
		KeyID:     "k",
		Hash:      "noop",
		ExpiresAt: &past,
	}
	err := rec.VerifySecret("noop")
	if !errors.Is(err, ErrAPIKeyExpired) {
		t.Fatalf("expected ErrAPIKeyExpired, got %v", err)
	}
}

func TestAPIKeyRecord_VerifyRejectsRevoked(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	rec := APIKeyRecord{
		KeyID:     "k",
		Hash:      "noop",
		RevokedAt: &past,
	}
	err := rec.VerifySecret("noop")
	if !errors.Is(err, ErrAPIKeyRevoked) {
		t.Fatalf("expected ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKeyRecord_HasCapability(t *testing.T) {
	rec := APIKeyRecord{
		FullAccess:   false,
		Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve},
	}
	if !rec.HasCapability(types.APIKeyCapabilityRetrieve) {
		t.Fatal("must have retrieve")
	}
	if rec.HasCapability(types.APIKeyCapabilityIngest) {
		t.Fatal("must NOT have ingest")
	}
}

func TestAPIKeyRecord_HasCapability_FullAccessGrantsAll(t *testing.T) {
	rec := APIKeyRecord{FullAccess: true}
	if !rec.HasCapability(types.APIKeyCapabilityRetrieve) {
		t.Fatal("FullAccess must imply retrieve")
	}
	if !rec.HasCapability(types.APIKeyCapabilityManageKnowledgeBases) {
		t.Fatal("FullAccess must imply manage_kbs")
	}
	if !rec.HasCapability(types.APIKeyCapabilitySystemSettingsManage) {
		t.Fatal("FullAccess must imply system_settings_manage")
	}
}

func TestAPIKeyRecord_HasCapability_EmptyCapability(t *testing.T) {
	rec := APIKeyRecord{Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}}
	if rec.HasCapability("") {
		t.Fatal("empty capability should never be granted")
	}
}

func TestAPIKeyRecord_Authorizes(t *testing.T) {
	rec := APIKeyRecord{Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}}
	if err := rec.Authorize(types.APIKeyCapabilityRetrieve); err != nil {
		t.Fatalf("authorize retrieve: %v", err)
	}
	if err := rec.Authorize(types.APIKeyCapabilityIngest); !errors.Is(err, ErrAPIKeyInsufficientCapability) {
		t.Fatalf("expected ErrAPIKeyInsufficientCapability, got %v", err)
	}
}

func TestAPIKeyRecord_Authorizes_FullAccessPasses(t *testing.T) {
	rec := APIKeyRecord{FullAccess: true}
	if err := rec.Authorize(types.APIKeyCapabilityManageKnowledgeBases); err != nil {
		t.Fatalf("full-access should authorize: %v", err)
	}
}

// ---------- APIKey verification: full pipeline (split → lookup → verify) ----------

func TestAPIKey_ResolveRecord_AcceptsValid(t *testing.T) {
	hash, _ := HashAPIKeySecret("secret-1")
	store := staticRecordStore{
		"ak_abc": {KeyID: "ak_abc", Hash: hash, Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}},
	}
	resolved, err := ResolveAPIKey("ak_abc.secret-1", store)
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if resolved.KeyID != "ak_abc" {
		t.Fatalf("resolved wrong keyID: %s", resolved.KeyID)
	}
}

func TestAPIKey_ResolveRecord_BadFormat(t *testing.T) {
	store := staticRecordStore{}
	if _, err := ResolveAPIKey("not-a-key", store); !errors.Is(err, ErrAPIKeyMalformed) {
		t.Fatalf("expected ErrAPIKeyMalformed, got %v", err)
	}
}

func TestAPIKey_ResolveRecord_NotFound(t *testing.T) {
	store := staticRecordStore{}
	if _, err := ResolveAPIKey("ak_ghost.zzz", store); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKey_ResolveRecord_BadSecret(t *testing.T) {
	hash, _ := HashAPIKeySecret("real")
	store := staticRecordStore{
		"ak_x": {KeyID: "ak_x", Hash: hash, Capabilities: []types.APIKeyCapability{types.APIKeyCapabilityRetrieve}},
	}
	if _, err := ResolveAPIKey("ak_x.fake", store); !errors.Is(err, ErrAPIKeySecretMismatch) {
		t.Fatalf("expected ErrAPIKeySecretMismatch, got %v", err)
	}
}

func TestAPIKey_ResolveRecord_ExpiredRejected(t *testing.T) {
	hash, _ := HashAPIKeySecret("real")
	past := time.Now().Add(-time.Hour)
	store := staticRecordStore{
		"ak_x": {
			KeyID:     "ak_x",
			Hash:      hash,
			ExpiresAt: &past,
		},
	}
	if _, err := ResolveAPIKey("ak_x.real", store); !errors.Is(err, ErrAPIKeyExpired) {
		t.Fatalf("expected ErrAPIKeyExpired, got %v", err)
	}
}

func TestAPIKey_ResolveRecord_RevokedRejected(t *testing.T) {
	hash, _ := HashAPIKeySecret("real")
	past := time.Now().Add(-time.Hour)
	store := staticRecordStore{
		"ak_x": {
			KeyID:     "ak_x",
			Hash:      hash,
			RevokedAt: &past,
		},
	}
	if _, err := ResolveAPIKey("ak_x.real", store); !errors.Is(err, ErrAPIKeyRevoked) {
		t.Fatalf("expected ErrAPIKeyRevoked, got %v", err)
	}
}

// staticRecordStore implements APIKeyLookup for tests.
type staticRecordStore map[string]APIKeyRecord

func (s staticRecordStore) GetByID(keyID string) (APIKeyRecord, bool) {
	r, ok := s[keyID]
	return r, ok
}
