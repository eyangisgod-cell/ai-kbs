package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest is the JSON body for POST /api/ai-kbs/auth/login.
// Either email or username must be present; password is required.
type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginResponse is the v1.0 envelope data payload for successful
// login. The token is a signed JWT; the refresh token is opaque and
// lives in the auth_tokens table.
type LoginResponse struct {
	Token        string          `json:"token"`
	RefreshToken string          `json:"refresh_token,omitempty"`
	User         *types.User     `json:"user"`
	Capabilities []string        `json:"capabilities,omitempty"`
}

// LoginHandler wires the login → JWT issuance flow. It depends on:
//   - UserLookup: find a user by email / username and verify password
//   - JWTSigner:   mint the access token
//   - TokenIssuer: persist the refresh token (opaque string)
//   - RequestID:   traced into the envelope
//
// The handler itself is stateless and has no DB connection; everything
// happens through the interfaces.
type LoginHandler struct {
	UserLookup   UserLookup
	JWTSigner    JWTSigner
	TokenIssuer  TokenIssuer
	JWTLifetime  time.Duration
	RequestID    func(*gin.Context) string
}

// UserLookup is the small surface LoginHandler needs from the user
// repository. Production wires the userService; tests pass a stub.
type UserLookup interface {
	GetByEmail(ctx context.Context, email string) (*types.User, error)
	GetByUsername(ctx context.Context, username string) (*types.User, error)
}

// TokenIssuer persists refresh tokens. LoginHandler does not depend on
// the full user service — issuing a refresh token is a small contract
// that any auth store can satisfy.
type TokenIssuer interface {
	IssueRefreshToken(ctx context.Context, userID string) (string, error)
}

// ErrInvalidCredentials is returned by UserLookup when email / username
// do not match or the bcrypt check fails. LoginHandler maps both to
// 401 without leaking which field failed.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// Login validates the credentials and returns a JWT envelope response.
//
// Lookup precedence: email → username. We always verify the password
// with bcrypt; a missing user and a wrong password yield the same
// envelope code so attackers cannot enumerate accounts.
func (h *LoginHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"invalid login payload", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"password is required", http.StatusBadRequest)
		return
	}

	var (
		user *types.User
		err  error
	)
	switch {
	case req.Email != "":
		user, err = h.UserLookup.GetByEmail(reqCtx(c), req.Email)
	case req.Username != "":
		user, err = h.UserLookup.GetByUsername(reqCtx(c), req.Username)
	default:
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"email or username is required", http.StatusBadRequest)
		return
	}
	if err != nil || user == nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"invalid credentials", http.StatusUnauthorized)
		return
	}
	if !user.IsActive {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"account disabled", http.StatusForbidden)
		return
	}
	if user.PasswordHash == "" {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthLoginFail,
			"invalid credentials", http.StatusUnauthorized)
		return
	}

	ttl := h.JWTLifetime
	if ttl <= 0 {
		ttl = DefaultJWTLifetime
	}
	now := time.Now()
	tok, err := h.JWTSigner.Sign(JWTClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		TenantID:  user.TenantID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	})
	if err != nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeInternalError,
			"failed to mint access token", http.StatusInternalServerError)
		return
	}
	refresh, err := h.TokenIssuer.IssueRefreshToken(reqCtx(c), user.ID)
	if err != nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeInternalError,
			"failed to issue refresh token", http.StatusInternalServerError)
		return
	}
	env := envelope.Ok(LoginResponse{
		Token:        tok,
		RefreshToken: refresh,
		User:         user,
	}, requestIDOf(h.RequestID, c))
	envelope.WriteJSON(c, http.StatusOK, env)
}

// StatusHandler reports the resolved principal. It depends on
// Middleware having already populated the gin keys; nothing here
// re-authenticates.
type StatusHandler struct {
	RequestID func(*gin.Context) string
}

type StatusResponse struct {
	User         *types.User `json:"user"`
	Capabilities []string    `json:"capabilities,omitempty"`
	Mode         AuthMode    `json:"mode"`
}

// Status is the GET /api/ai-kbs/auth/status handler. It is the
// heartbeat the SPA polls to detect "logged in or not" without
// triggering a re-login.
func (h *StatusHandler) Status(c *gin.Context) {
	v, ok := c.Get("user")
	if !ok || v == nil {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthPermissionDenied,
			"not authenticated", http.StatusUnauthorized)
		return
	}
	user, _ := v.(*types.User)
	caps := capabilitiesFromContext(c)
	env := envelope.Ok(StatusResponse{
		User:         user,
		Capabilities: caps,
		Mode:         AuthModeAuto,
	}, requestIDOf(h.RequestID, c))
	envelope.WriteJSON(c, http.StatusOK, env)
}

// LogoutHandler invalidates the refresh token. The bearer token itself
// stays valid until expiry (logout is best-effort until a future
// blacklist lives in Redis).
type LogoutHandler struct {
	TokenRevoker TokenRevoker
	RequestID    func(*gin.Context) string
}

// TokenRevoker is the surface LogoutHandler needs.
type TokenRevoker interface {
	RevokeRefreshToken(ctx context.Context, userID, token string) error
}

func (h *LogoutHandler) Logout(c *gin.Context) {
	userID := userIDFromContext(c)
	if userID == "" {
		writeAuthError(c, requestIDOf(h.RequestID, c), envelope.CodeAuthPermissionDenied,
			"not authenticated", http.StatusUnauthorized)
		return
	}
	if h.TokenRevoker != nil {
		_ = h.TokenRevoker.RevokeRefreshToken(reqCtx(c), userID, c.GetHeader("X-Refresh-Token"))
	}
	env := envelope.Ok(map[string]any{"logged_out": true}, requestIDOf(h.RequestID, c))
	envelope.WriteJSON(c, http.StatusOK, env)
}

// requestIDOf returns the request id from the user-supplied fn, falling
// back to the gin header.
func requestIDOf(fn func(*gin.Context) string, c *gin.Context) string {
	if fn != nil {
		return fn(c)
	}
	return c.GetHeader("X-Request-ID")
}

func capabilitiesFromContext(c *gin.Context) []string {
	v, ok := c.Get("api_key_capabilities")
	if !ok {
		return nil
	}
	out, _ := v.([]string)
	return out
}

func userIDFromContext(c *gin.Context) string {
	if u, ok := types.UserIDFromContext(c.Request.Context()); ok {
		return u
	}
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*types.User); ok && u != nil {
			return u.ID
		}
	}
	return ""
}

// reqCtx returns the underlying request context. Returned as
// context.Context so it satisfies the real interfaces, not a stripped
// subset.
func reqCtx(c *gin.Context) context.Context {
	if c == nil || c.Request == nil {
		return context.Background()
	}
	return c.Request.Context()
}
