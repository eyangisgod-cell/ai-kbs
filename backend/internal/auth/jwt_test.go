package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWT_HS256_RoundTrip(t *testing.T) {
	signer := NewHS256([]byte("secret"))
	claims := JWTClaims{
		UserID:    "u1",
		Username:  "alice",
		Email:     "alice@example.com",
		TenantID:  7,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	tok, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parsed, err := signer.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.UserID != "u1" || parsed.TenantID != 7 {
		t.Fatalf("round-trip mismatch: %+v", parsed)
	}
}

func TestJWT_HS256_RejectsExpired(t *testing.T) {
	signer := NewHS256([]byte("secret"))
	claims := JWTClaims{
		UserID:    "u1",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		IssuedAt:  time.Now().Add(-2 * time.Hour).Unix(),
	}
	tok, _ := signer.Sign(claims)
	_, err := signer.Parse(tok)
	if !errors.Is(err, ErrJWTExpired) {
		t.Fatalf("expected ErrJWTExpired, got %v", err)
	}
}

func TestJWT_HS256_RejectsBadSignature(t *testing.T) {
	signer := NewHS256([]byte("secret"))
	claims := JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	tok, _ := signer.Sign(claims)

	other := NewHS256([]byte("wrong"))
	if _, err := other.Parse(tok); err == nil {
		t.Fatal("expected error on different secret")
	}
}

func TestJWT_HS256_RejectsMalformedToken(t *testing.T) {
	signer := NewHS256([]byte("secret"))
	if _, err := signer.Parse("not-a-jwt"); !errors.Is(err, ErrJWTInvalid) {
		t.Fatalf("expected ErrJWTInvalid, got %v", err)
	}
}

func TestJWT_HS256_RejectsNonHSToken(t *testing.T) {
	// Build an RS256 token and try parsing with HS256 signer.
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	str, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("build RS256: %v", err)
	}
	signer := NewHS256([]byte("secret"))
	if _, err := signer.Parse(str); err == nil {
		t.Fatal("expected error when RS256 token is fed into HS256 verifier")
	}
}

func TestJWT_RS256_RoundTrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa: %v", err)
	}
	signer := NewRS256(priv, &priv.PublicKey)
	claims := JWTClaims{
		UserID: "u1", Email: "u@x.com",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	tok, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parsed, err := signer.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.UserID != "u1" {
		t.Fatalf("round-trip: %+v", parsed)
	}
}

func TestJWT_RS256_RejectsExpired(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer := NewRS256(priv, &priv.PublicKey)
	claims := JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	tok, _ := signer.Sign(claims)
	_, err := signer.Parse(tok)
	if !errors.Is(err, ErrJWTExpired) {
		t.Fatalf("expected ErrJWTExpired, got %v", err)
	}
}

func TestJWT_RS256_RejectsHS256Token(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	// HS256 token, RS256 verifier must reject
	hs := NewHS256([]byte("k"))
	tok, _ := hs.Sign(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	rs := NewRS256(priv, &priv.PublicKey)
	if _, err := rs.Parse(tok); err == nil {
		t.Fatal("expected RS256 verifier to reject HS256 token")
	}
}

func TestJWT_RS256_RejectsTamperedPayload(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer := NewRS256(priv, &priv.PublicKey)
	tok, _ := signer.Sign(JWTClaims{UserID: "u1", TenantID: 1, ExpiresAt: time.Now().Add(time.Hour).Unix()})
	// Tamper: change a character in the payload segment
	parts := splitJWT(tok)
	if len(parts) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(parts))
	}
	// Flip a char in the payload
	if parts[1][0] == 'A' {
		parts[1] = "B" + parts[1][1:]
	} else {
		parts[1] = "A" + parts[1][1:]
	}
	bad := parts[0] + "." + parts[1] + "." + parts[2]
	if _, err := signer.Parse(bad); !errors.Is(err, ErrJWTInvalid) {
		t.Fatalf("expected ErrJWTInvalid on tampered payload, got %v", err)
	}
}

func TestJWT_RS256_VerifierOnly(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	// Sign with a full signer, parse with a verify-only signer
	signer := NewRS256(priv, &priv.PublicKey)
	tok, _ := signer.Sign(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	verifier := NewRS256Verifier(&priv.PublicKey)
	if _, err := verifier.Parse(tok); err != nil {
		t.Fatalf("verifier-only parse: %v", err)
	}
	// Verify-only signer cannot sign
	if _, err := verifier.Sign(JWTClaims{}); err == nil {
		t.Fatal("verifier-only signer must not sign")
	}
}

func TestJWT_HS256_RejectsEmptyToken(t *testing.T) {
	signer := NewHS256([]byte("k"))
	if _, err := signer.Parse(""); !errors.Is(err, ErrJWTInvalid) {
		t.Fatalf("expected ErrJWTInvalid, got %v", err)
	}
}

func TestJWT_HS256_AcceptsLifetimeInPast(t *testing.T) {
	signer := NewHS256([]byte("k"))
	claims := JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(15 * time.Minute).Unix()}
	tok, _ := signer.Sign(claims)
	parsed, err := signer.Parse(tok)
	if err != nil {
		t.Fatalf("fresh token must parse: %v", err)
	}
	if parsed.UserID != "u1" {
		t.Fatalf("claims dropped: %+v", parsed)
	}
}

func TestJWT_HS256_RequiredFields(t *testing.T) {
	signer := NewHS256([]byte("k"))
	// Missing sub (UserID) and exp — both required by parseClaims.
	claims := JWTClaims{UserID: "", ExpiresAt: 0}
	tok, _ := signer.Sign(claims)
	if _, err := signer.Parse(tok); err == nil {
		t.Fatal("expected error when required claims missing")
	}
}

// splitJWT returns the dot-separated segments of a JWT.
func splitJWT(s string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(s[i])
	}
	out = append(out, cur)
	return out
}
