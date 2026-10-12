package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims is the AI-KBS claim set carried inside every access token.
//
// The contract is deliberately minimal: UserID + TenantID + a few
// profile fields + standard time bounds (iat / exp / nbf). Adding new
// fields is wire-compatible; removing a field is breaking and must
// follow the envelope v1.0 versioning rules.
type JWTClaims struct {
	UserID   string `json:"sub"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	TenantID uint64 `json:"tenant_id,omitempty"`

	IssuedAt  int64 `json:"iat,omitempty"`
	ExpiresAt int64 `json:"exp,omitempty"`
	NotBefore int64 `json:"nbf,omitempty"`
}

// JWTSigner is the union of Sign and Parse. Production callers (e.g.
// login handlers) use a Signer; downstream verifiers (e.g. middleware)
// receive a JWTSigner that may be verify-only.
type JWTSigner interface {
	Sign(claims JWTClaims) (string, error)
	Parse(tokenString string) (*JWTClaims, error)
}

// Errors for the JWT path. These map to envelope.CodeAuthTokenExpired
// (401) and the verification failure falls through to internal.error.
var (
	ErrJWTExpired = errors.New("jwt: token expired")
	ErrJWTInvalid = errors.New("jwt: token invalid")
	ErrJWTSigning = errors.New("jwt: signing error")
	ErrJWTNotYet  = errors.New("jwt: token not yet valid")
)

// jwtCommonClaims is the minimal claim set the golang-jwt library
// understands for time-bound verification. We always populate exp /
// iat / nbf; "sub" carries the user ID and is required (Parse rejects
// when missing).
type jwtCommonClaims struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	TenantID uint64 `json:"tenant_id,omitempty"`
	jwt.RegisteredClaims
}

// hs256Signer is the HMAC-SHA256 implementation of JWTSigner.
type hs256Signer struct {
	secret []byte
}

func NewHS256(secret []byte) JWTSigner {
	cp := make([]byte, len(secret))
	copy(cp, secret)
	return &hs256Signer{secret: cp}
}

func (s *hs256Signer) Sign(claims JWTClaims) (string, error) {
	if s == nil || len(s.secret) == 0 {
		return "", ErrJWTSigning
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, s.toCommon(claims))
	return tok.SignedString(s.secret)
}

func (s *hs256Signer) Parse(tokenString string) (*JWTClaims, error) {
	return parseWithKeyfunc(tokenString, jwt.SigningMethodHS256.Alg(), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrJWTInvalid
		}
		return s.secret, nil
	})
}

func (s *hs256Signer) toCommon(c JWTClaims) *jwtCommonClaims {
	now := time.Now()
	if c.IssuedAt == 0 {
		c.IssuedAt = now.Unix()
	}
	if c.NotBefore == 0 {
		c.NotBefore = now.Unix()
	}
	if c.ExpiresAt == 0 {
		c.ExpiresAt = now.Add(DefaultJWTLifetime).Unix()
	}
	return &jwtCommonClaims{
		Username: c.Username,
		Email:    c.Email,
		TenantID: c.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID,
			IssuedAt:  jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)),
			ExpiresAt: jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)),
			NotBefore: jwt.NewNumericDate(time.Unix(c.NotBefore, 0)),
		},
	}
}

// DefaultJWTLifetime is the access-token lifetime applied when the
// caller does not specify ExpiresAt. 15 minutes matches the
// /auth/login issuance policy and keeps the refresh-token path busy.
const DefaultJWTLifetime = 15 * time.Minute

// rs256Signer is the RSA-SHA256 implementation of JWTSigner. It is
// split from hs256Signer because the symmetric / asymmetric proofs use
// entirely different key material — keeping the structs separate makes
// the "this verifier accepts which alg" guarantee trivially auditable.
type rs256Signer struct {
	priv *rsa.PrivateKey
	pub  *rsa.PublicKey
}

func NewRS256(priv *rsa.PrivateKey, pub *rsa.PublicKey) JWTSigner {
	return &rs256Signer{priv: priv, pub: pub}
}

// NewRS256Verifier returns a verify-only signer. Useful for middleware
// where the signing key must never reach the request goroutine.
func NewRS256Verifier(pub *rsa.PublicKey) JWTSigner {
	return &rs256Signer{priv: nil, pub: pub}
}

func (s *rs256Signer) Sign(claims JWTClaims) (string, error) {
	if s == nil || s.priv == nil {
		return "", ErrJWTSigning
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, s.toCommon(claims))
	return tok.SignedString(s.priv)
}

func (s *rs256Signer) Parse(tokenString string) (*JWTClaims, error) {
	return parseWithKeyfunc(tokenString, jwt.SigningMethodRS256.Alg(), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, ErrJWTInvalid
		}
		if s.pub == nil {
			return nil, ErrJWTInvalid
		}
		return s.pub, nil
	})
}

func (s *rs256Signer) toCommon(c JWTClaims) *jwtCommonClaims {
	now := time.Now()
	if c.IssuedAt == 0 {
		c.IssuedAt = now.Unix()
	}
	if c.NotBefore == 0 {
		c.NotBefore = now.Unix()
	}
	if c.ExpiresAt == 0 {
		c.ExpiresAt = now.Add(DefaultJWTLifetime).Unix()
	}
	return &jwtCommonClaims{
		Username: c.Username,
		Email:    c.Email,
		TenantID: c.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID,
			IssuedAt:  jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)),
			ExpiresAt: jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)),
			NotBefore: jwt.NewNumericDate(time.Unix(c.NotBefore, 0)),
		},
	}
}

// parseWithKeyfunc is the shared verifier body used by both signer
// flavours. It enforces:
//   - alg matches the expected signer type
//   - exp / nbf are present (auth.token.expired if expired)
//   - sub (UserID) is present
//   - the token verifies under the configured key
func parseWithKeyfunc(tokenString string, alg string, kf jwt.Keyfunc) (*JWTClaims, error) {
	if tokenString == "" {
		return nil, ErrJWTInvalid
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{alg}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	var raw jwtCommonClaims
	tok, err := parser.ParseWithClaims(tokenString, &raw, kf)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrJWTExpired
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, ErrJWTNotYet
		default:
			return nil, ErrJWTInvalid
		}
	}
	if tok == nil || !tok.Valid {
		return nil, ErrJWTInvalid
	}
	if raw.Subject == "" {
		return nil, ErrJWTInvalid
	}
	out := &JWTClaims{
		UserID:   raw.Subject,
		Username: raw.Username,
		Email:    raw.Email,
		TenantID: raw.TenantID,
	}
	if raw.IssuedAt != nil {
		out.IssuedAt = raw.IssuedAt.Unix()
	}
	if raw.ExpiresAt != nil {
		out.ExpiresAt = raw.ExpiresAt.Unix()
	}
	if raw.NotBefore != nil {
		out.NotBefore = raw.NotBefore.Unix()
	}
	return out, nil
}
