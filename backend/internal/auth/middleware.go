package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
)

// timeNow / windowOrDefault are tiny indirections that let tests freeze
// the clock; production callers don't touch them.
var (
	timeNow           = func() time.Time { return time.Now() }
	windowOrDefault   = func(w time.Duration) time.Duration {
		if w <= 0 {
			return DefaultHMACWindow
		}
		return w
	}
)

// AuthMode is how the caller wants the middleware to authenticate this
// request. Use AuthModeAuto (the default) to let the header choose
// between HMAC / Bearer / APIKey.
type AuthMode string

const (
	AuthModeAuto   AuthMode = "auto"
	AuthModeHMAC   AuthMode = "hmac"
	AuthModeJWT    AuthMode = "jwt"
	AuthModeAPIKey AuthMode = "apikey"
)

// apiKeyVerifyFailedMessage is the single user-facing message returned
// for any API-key credential problem that could otherwise leak whether a
// key ID exists (malformed / unknown / bad-secret). The wire envelope
// error.code stays distinct so internal observability is unaffected.
const apiKeyVerifyFailedMessage = "api key verification failed"

// PrincipalResolver is the dependency the middleware needs to turn an
// authenticated credential into a *types.User. Repositories or services
// implement it; tests pass a stub.
type PrincipalResolver interface {
	// UserFromHMAC maps a key ID to the principal identity. Returns
	// (nil, ErrPrincipalNotFound) when no user is bound to the key.
	UserFromHMAC(ctx context.Context, keyID string) (*types.User, error)
	// UserFromJWT is called after the JWT claims have been parsed.
	// Implementations usually look up the user row to confirm it
	// hasn't been disabled since the token was minted.
	UserFromJWT(ctx context.Context, claims *JWTClaims) (*types.User, error)
	// UserFromAPIKey returns the user bound to the API key's tenant
	// (or a synthetic system-N user when the tenant has no human
	// user, mirroring the WeKnora convention).
	UserFromAPIKey(ctx context.Context, rec APIKeyRecord) (*types.User, error)
}

// ErrPrincipalNotFound is returned by PrincipalResolver when no user
// matches the credential. The middleware translates it to 401.
var ErrPrincipalNotFound = errors.New("auth: principal not found")

// MiddlewareConfig groups the wiring the middleware needs. Tests can
// supply narrow fakes; production wires real repositories.
type MiddlewareConfig struct {
	Mode          AuthMode
	HMACSecret    []byte
	HMACWindow    time.Duration
	NonceStore    NonceStore
	JWTSigner     JWTSigner
	APIKeyLookup  APIKeyLookup
	Principal     PrincipalResolver
	RequestID     func(*gin.Context) string
	// MaxBody caps the body read for HMAC verification; oversized
	// requests get ErrBodyTooLarge → 400. Zero means 1 MiB.
	MaxBody int64
}

// Middleware returns a Gin handler that authenticates the request
// using the configured credential family and injects the resolved
// principal into the Gin / request contexts (see Inject).
//
// On failure the middleware writes the error as a v1.0 envelope with
// the appropriate HTTP status; no further handlers run.
func Middleware(cfg MiddlewareConfig) gin.HandlerFunc {
	if cfg.Mode == "" {
		cfg.Mode = AuthModeAuto
	}
	if cfg.RequestID == nil {
		cfg.RequestID = defaultRequestID
	}
	return func(c *gin.Context) {
		switch cfg.Mode {
		case AuthModeHMAC:
			authHMAC(c, cfg)
		case AuthModeJWT:
			authJWT(c, cfg)
		case AuthModeAPIKey:
			authAPIKey(c, cfg)
		default:
			authAuto(c, cfg)
		}
	}
}

// authAuto tries HMAC → JWT → APIKey in order, picking the first one
// that matches the Authorization header shape.
func authAuto(c *gin.Context, cfg MiddlewareConfig) {
	hdr := c.GetHeader("Authorization")
	switch {
	case strings.HasPrefix(hdr, "HMAC "):
		authHMAC(c, cfg)
	case strings.HasPrefix(hdr, "Bearer "):
		authJWT(c, cfg)
	case strings.HasPrefix(hdr, "APIKey "):
		authAPIKey(c, cfg)
	default:
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"missing or unrecognized Authorization header", http.StatusUnauthorized)
	}
}

func authHMAC(c *gin.Context, cfg MiddlewareConfig) {
	if cfg.HMACSecret == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeInternalError,
			"hmac secret not configured", http.StatusInternalServerError)
		return
	}
	if err := VerifyRequest(c, HMACVerifyConfig{
		Secret:       cfg.HMACSecret,
		Now:          timeNow(),
		Window:       windowOrDefault(cfg.HMACWindow),
		NonceStore:   cfg.NonceStore,
		MaxBodyBytes: cfg.MaxBody,
	}); err != nil {
		switch {
		case errors.Is(err, ErrTimestampOutOfRange):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthTokenExpired,
				"hmac timestamp out of window", http.StatusUnauthorized)
		case errors.Is(err, ErrNonceReplay):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthTokenExpired,
				"hmac nonce replay", http.StatusUnauthorized)
		case errors.Is(err, ErrMalformedHeader):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				"malformed hmac authorization header", http.StatusUnauthorized)
		case errors.Is(err, ErrBodyTooLarge):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				"hmac body too large", http.StatusBadRequest)
		default:
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				"hmac signature mismatch", http.StatusForbidden)
		}
		return
	}
	// Resolve principal from key ID.
	h, _ := ParseHMACHeader(c.GetHeader("Authorization"))
	user, err := cfg.Principal.UserFromHMAC(c.Request.Context(), h.KeyID)
	if err != nil || user == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"unknown hmac key", http.StatusUnauthorized)
		return
	}
	Inject(c, user, nil)
}

func authJWT(c *gin.Context, cfg MiddlewareConfig) {
	if cfg.JWTSigner == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeInternalError,
			"jwt signer not configured", http.StatusInternalServerError)
		return
	}
	tok, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || tok == "" {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"missing bearer token", http.StatusUnauthorized)
		return
	}
	claims, err := cfg.JWTSigner.Parse(tok)
	if err != nil {
		switch {
		case errors.Is(err, ErrJWTExpired):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthTokenExpired,
				"jwt expired", http.StatusUnauthorized)
		default:
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				"jwt invalid", http.StatusUnauthorized)
		}
		return
	}
	user, err := cfg.Principal.UserFromJWT(c.Request.Context(), claims)
	if err != nil || user == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"jwt principal not found", http.StatusUnauthorized)
		return
	}
	Inject(c, user, nil)
}

func authAPIKey(c *gin.Context, cfg MiddlewareConfig) {
	if cfg.APIKeyLookup == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeInternalError,
			"api key store not configured", http.StatusInternalServerError)
		return
	}
	raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "APIKey ")
	if !ok || raw == "" {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"missing api key", http.StatusUnauthorized)
		return
	}
	rec, err := ResolveAPIKey(raw, cfg.APIKeyLookup)
	if err != nil {
		// Malformed / unknown-key / bad-secret all collapse to a single
		// user-facing message AND a single HTTP status so an attacker
		// cannot probe whether a key exists by comparing the wire text
		// or status code. The internal envelope error.code stays distinct
		// so ops can still distinguish the three paths in logs.
		switch {
		case errors.Is(err, ErrAPIKeyMalformed):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				apiKeyVerifyFailedMessage, http.StatusUnauthorized)
		case errors.Is(err, ErrAPIKeyNotFound):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				apiKeyVerifyFailedMessage, http.StatusUnauthorized)
		case errors.Is(err, ErrAPIKeyRevoked):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthAPIKeyRevoked,
				"api key revoked", http.StatusUnauthorized)
		case errors.Is(err, ErrAPIKeyExpired):
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthAPIKeyRevoked,
				"api key expired", http.StatusUnauthorized)
		default:
			writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
				apiKeyVerifyFailedMessage, http.StatusUnauthorized)
		}
		return
	}
	user, err := cfg.Principal.UserFromAPIKey(c.Request.Context(), rec)
	if err != nil || user == nil {
		writeAuthError(c, cfg.RequestID(c), envelope.CodeAuthPermissionDenied,
			"api key principal not found", http.StatusUnauthorized)
		return
	}
	caps := make([]string, 0, len(rec.Capabilities))
	for _, c := range rec.Capabilities {
		caps = append(caps, string(c))
	}
	Inject(c, user, caps)
}

// Inject attaches the resolved principal to BOTH the Gin keys map and
// the request's context.Context (mirroring the platform's authSession
// invariant — see middleware/auth_context.go). The capabilities slice
// may be nil for non-API-key principals.
func Inject(c *gin.Context, user *types.User, capabilities []string) {
	if user != nil {
		c.Set("user", user)
		c.Set(types.UserContextKey.String(), user)
		ctx := context.WithValue(c.Request.Context(), types.UserContextKey, user)
		c.Request = c.Request.WithContext(ctx)
	}
	if capabilities != nil {
		c.Set("api_key_capabilities", capabilities)
	}
}

// writeAuthError renders a v1.0 envelope response. requestID is taken
// from the gin context when present; fallback to empty keeps tests
// from coupling to the X-Request-ID middleware ordering.
func writeAuthError(c *gin.Context, requestID string, code envelope.Code, message string, status int) {
	env := envelope.New(code, message, requestID)
	envelope.WriteJSON(c, status, env)
	c.Abort()
}

func defaultRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v := c.GetHeader("X-Request-ID"); v != "" {
		return v
	}
	return ""
}
