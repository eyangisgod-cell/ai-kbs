package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ---------- user-lookup / token-issuer test doubles ----------

type stubUserLookup struct {
	byEmail    map[string]*types.User
	byUsername map[string]*types.User
}

func (s *stubUserLookup) GetByEmail(_ context.Context, email string) (*types.User, error) {
	u, ok := s.byEmail[email]
	if !ok {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

func (s *stubUserLookup) GetByUsername(_ context.Context, name string) (*types.User, error) {
	u, ok := s.byUsername[name]
	if !ok {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

type stubTokenIssuer struct {
	tokens map[string]string
}

func (s *stubTokenIssuer) IssueRefreshToken(_ context.Context, userID string) (string, error) {
	tok := "rt-" + userID
	if s.tokens == nil {
		s.tokens = map[string]string{}
	}
	s.tokens[userID] = tok
	return tok, nil
}

type stubRevoker struct{}

func (stubRevoker) RevokeRefreshToken(_ context.Context, _, _ string) error { return nil }

func newUser(t *testing.T, id, email, password string, active bool) *types.User {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return &types.User{
		ID:           id,
		Username:     email,
		Email:        email,
		PasswordHash: string(h),
		IsActive:     active,
		TenantID:     7,
	}
}

func callJSON(h gin.HandlerFunc, body any) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", h)
	r.POST("/logout", h)
	r.GET("/status", h)
	var reader *bytes.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest("POST", "/login", reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------- login handler tests ----------

func TestLogin_AcceptsEmailAndPassword(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", true)
	lookup := &stubUserLookup{byEmail: map[string]*types.User{user.Email: user}}
	issuer := &stubTokenIssuer{}
	signer := NewHS256([]byte("k"))

	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   signer,
		TokenIssuer: issuer,
		RequestID:   func(*gin.Context) string { return "rid-1" },
	}
	w := callJSON(h.Login, LoginRequest{Email: user.Email, Password: "password-1!"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var env envelope.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Success {
		t.Fatalf("success=false, body=%s", w.Body.String())
	}
	if env.RequestID != "rid-1" {
		t.Fatalf("request id = %q, want rid-1", env.RequestID)
	}
	if env.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", env.ExitCode)
	}
	// Data is a LoginResponse struct
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is not a JSON object: %T", env.Data)
	}
	if _, ok := data["token"].(string); !ok || data["token"].(string) == "" {
		t.Fatalf("login response missing token: %+v", data)
	}
}

func TestLogin_AcceptsUsername(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", true)
	lookup := &stubUserLookup{byUsername: map[string]*types.User{user.Username: user}}
	issuer := &stubTokenIssuer{}
	signer := NewHS256([]byte("k"))

	h := &LoginHandler{UserLookup: lookup, JWTSigner: signer, TokenIssuer: issuer}
	w := callJSON(h.Login, LoginRequest{Username: user.Username, Password: "password-1!"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestLogin_RejectsBadPassword(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", true)
	lookup := &stubUserLookup{byEmail: map[string]*types.User{user.Email: user}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	w := callJSON(h.Login, LoginRequest{Email: user.Email, Password: "wrong-pw"})
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusUnauthorized)
}

func TestLogin_RejectsUnknownEmail(t *testing.T) {
	lookup := &stubUserLookup{byEmail: map[string]*types.User{}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	w := callJSON(h.Login, LoginRequest{Email: "ghost@example.com", Password: "whatever-1"})
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusUnauthorized)
}

func TestLogin_RejectsDisabledAccount(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", false)
	lookup := &stubUserLookup{byEmail: map[string]*types.User{user.Email: user}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	w := callJSON(h.Login, LoginRequest{Email: user.Email, Password: "password-1!"})
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusForbidden)
}

func TestLogin_RejectsEmptyPassword(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", true)
	lookup := &stubUserLookup{byEmail: map[string]*types.User{user.Email: user}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	w := callJSON(h.Login, LoginRequest{Email: user.Email, Password: ""})
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusBadRequest)
}

func TestLogin_RejectsMissingEmailAndUsername(t *testing.T) {
	lookup := &stubUserLookup{byEmail: map[string]*types.User{}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	w := callJSON(h.Login, LoginRequest{Password: "password-1!"})
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusBadRequest)
}

func TestLogin_RejectsMalformedJSON(t *testing.T) {
	lookup := &stubUserLookup{byEmail: map[string]*types.User{}}
	h := &LoginHandler{
		UserLookup:  lookup,
		JWTSigner:   NewHS256([]byte("k")),
		TokenIssuer: &stubTokenIssuer{},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", h.Login)
	req := httptest.NewRequest("POST", "/login", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertEnvelope(t, w, envelope.CodeAuthLoginFail, http.StatusBadRequest)
}

func TestLogin_MintsValidJWT(t *testing.T) {
	user := newUser(t, "u1", "alice@example.com", "password-1!", true)
	lookup := &stubUserLookup{byEmail: map[string]*types.User{user.Email: user}}
	issuer := &stubTokenIssuer{}
	signer := NewHS256([]byte("k"))

	h := &LoginHandler{UserLookup: lookup, JWTSigner: signer, TokenIssuer: issuer}
	w := callJSON(h.Login, LoginRequest{Email: user.Email, Password: "password-1!"})

	var env envelope.Envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	data := env.Data.(map[string]any)
	tokStr := data["token"].(string)

	parsed, err := signer.Parse(tokStr)
	if err != nil {
		t.Fatalf("minted token must parse, got %v", err)
	}
	if parsed.UserID != user.ID {
		t.Fatalf("minted token user_id = %q, want %q", parsed.UserID, user.ID)
	}
	if parsed.TenantID != user.TenantID {
		t.Fatalf("minted token tenant = %d, want %d", parsed.TenantID, user.TenantID)
	}
	if parsed.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("minted token already expired")
	}
}

// ---------- status handler tests ----------

func TestStatus_RejectsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &StatusHandler{RequestID: func(*gin.Context) string { return "rid-1" }}
	r.GET("/status", h.Status)
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestStatus_ReturnsUserAndCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &types.User{ID: "u1", Username: "alice"})
		c.Set("api_key_capabilities", []string{"retrieve", "ingest"})
		c.Next()
	})
	h := &StatusHandler{RequestID: func(*gin.Context) string { return "rid-1" }}
	r.GET("/status", h.Status)
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var env envelope.Envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	data := env.Data.(map[string]any)
	if data["user"] == nil {
		t.Fatal("status payload missing user")
	}
	caps := data["capabilities"].([]any)
	if len(caps) != 2 {
		t.Fatalf("expected 2 caps, got %v", caps)
	}
}

// ---------- logout handler tests ----------

func TestLogout_RejectsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &LogoutHandler{TokenRevoker: stubRevoker{}, RequestID: func(*gin.Context) string { return "rid-1" }}
	r.POST("/logout", h.Logout)
	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertEnvelope(t, w, envelope.CodeAuthPermissionDenied, http.StatusUnauthorized)
}

func TestLogout_AcceptsAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &types.User{ID: "u1"})
		ctx := context.WithValue(c.Request.Context(), types.UserContextKey, &types.User{ID: "u1"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	h := &LogoutHandler{TokenRevoker: stubRevoker{}, RequestID: func(*gin.Context) string { return "rid-1" }}
	r.POST("/logout", h.Logout)
	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var env envelope.Envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if !env.Success {
		t.Fatalf("logout should be success, body=%s", w.Body.String())
	}
}

// Suppress "imported but not used" when only some helpers are exercised.
var _ = time.Second
