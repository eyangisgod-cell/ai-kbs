// Package envelope provides the frozen v1.0 envelope contract used by every
// API response. See ../spec/backend/error-handling.md and design.md §3.1.
package envelope

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEnvelope_Success_JSON verifies the success branch matches design.md §3.1.
func TestEnvelope_Success_JSON(t *testing.T) {
	env := Ok(map[string]any{"new_version": int64(7)}, "req-abc")
	if env.Success != true {
		t.Fatalf("success: want true, got false")
	}
	if env.ExitCode != 0 {
		t.Fatalf("exit_code: want 0, got %d", env.ExitCode)
	}
	if env.Error != nil {
		t.Fatalf("error: want nil on success, got %+v", env.Error)
	}
	if env.RequestID != "req-abc" {
		t.Fatalf("request_id: want req-abc, got %q", env.RequestID)
	}
	if env.Data == nil {
		t.Fatalf("data: want payload, got nil")
	}

	// JSON shape: required keys present, frozen field set respected.
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	for _, k := range []string{`"success":true`, `"exit_code":0`, `"data":`, `"error":null`, `"request_id":"req-abc"`} {
		if !strings.Contains(got, k) {
			t.Fatalf("missing %q in success JSON: %s", k, got)
		}
	}
	// trace_id and meta omitted on success w/o OTLP
	if strings.Contains(got, `"trace_id"`) {
		t.Fatalf("trace_id should be omitted when empty, got: %s", got)
	}
	if strings.Contains(got, `"meta"`) {
		t.Fatalf("meta should be omitted when empty, got: %s", got)
	}
}

// TestEnvelope_Error_JSON verifies the failure branch matches AC-B7.
func TestEnvelope_Error_JSON(t *testing.T) {
	env := New(CodeAuthLoginFail, "bad credentials", "req-xyz")
	if env.Success != false {
		t.Fatalf("success: want false, got true")
	}
	if env.ExitCode == 0 {
		t.Fatalf("exit_code: want non-zero on failure, got 0")
	}
	if env.Error == nil {
		t.Fatalf("error: want non-nil on failure, got nil")
	}
	if env.Error.Code != "auth.login.fail" {
		t.Fatalf("error.code: want auth.login.fail, got %q", env.Error.Code)
	}
	if env.Error.Message != "bad credentials" {
		t.Fatalf("error.message: want bad credentials, got %q", env.Error.Message)
	}
	if env.RequestID != "req-xyz" {
		t.Fatalf("request_id: want req-xyz, got %q", env.RequestID)
	}

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	// AC-B7: {success:false, exit_code, error:{code,message}, data:null, request_id}
	for _, k := range []string{
		`"success":false`,
		`"exit_code":`,
		`"error":{"code":"auth.login.fail","message":"bad credentials"}`,
		`"data":null`,
		`"request_id":"req-xyz"`,
	} {
		if !strings.Contains(got, k) {
			t.Fatalf("missing %q in error JSON: %s", k, got)
		}
	}
}

// TestEnvelope_RoundTrip guards the frozen v1.0 schema: every required field
// survives encode→decode without losing types or names.
func TestEnvelope_RoundTrip(t *testing.T) {
	in := Envelope{
		Success:   false,
		ExitCode:  22,
		Data:      nil,
		Error:     &ErrDetail{Code: "doc.version_conflict", Message: "stale"},
		RequestID: "01J0TEST",
		TraceID:   "0af7651916cd43dd8448eb211c80319c",
		Meta:      map[string]any{"current_version": int64(9)},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Envelope
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Success != in.Success || out.ExitCode != in.ExitCode ||
		out.RequestID != in.RequestID || out.TraceID != in.TraceID {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}
	if out.Error == nil || out.Error.Code != "doc.version_conflict" {
		t.Fatalf("error.code lost on roundtrip: %+v", out.Error)
	}
	if v, ok := out.Meta["current_version"].(float64); !ok || int64(v) != 9 {
		t.Fatalf("meta.current_version lost on roundtrip: %+v", out.Meta["current_version"])
	}
}

// TestEnvelope_Invariants protects the success/failure invariants from design.md §3.1.
func TestEnvelope_Invariants(t *testing.T) {
	// success==true ⇒ exit_code==0, error==nil
	if env := Ok(nil, "rid"); env.ExitCode != 0 || env.Error != nil {
		t.Fatalf("Ok() invariant broken: %+v", env)
	}
	// success==false ⇒ exit_code!=0 (SetExitCode ensures)
	env := New(CodeDocVersionConflict, "stale", "rid")
	if env.ExitCode == 0 {
		t.Fatalf("New() must produce non-zero exit_code, got 0")
	}
}

// TestMapStatus covers the canonical HTTP mapping table in error-handling.md.
func TestMapStatus(t *testing.T) {
	cases := map[string]int{
		CodeAuthLoginFail:       http.StatusUnauthorized,
		CodeAuthTokenExpired:    http.StatusUnauthorized,
		CodeAuthAPIKeyRevoked:   http.StatusUnauthorized,
		CodeRBACPermissionDenied: http.StatusForbidden,
		CodeDocNotFound:         http.StatusNotFound,
		CodeDocVersionConflict:  http.StatusConflict,
		CodeDocCodeMalformed:    http.StatusUnprocessableEntity,
		CodeBindingDuplicate:    http.StatusConflict,
		CodeSyncPartialFail:     http.StatusMultiStatus,
		CodeSyncCASUnresolved:   http.StatusConflict,
		CodeInternalError:       http.StatusInternalServerError,
		CodeInternalTimeout:     http.StatusGatewayTimeout,
	}
	for code, want := range cases {
		if got := MapStatus(code); got != want {
			t.Errorf("MapStatus(%q): want %d, got %d", code, want, got)
		}
	}
	// Unknown code → 500 (fail-closed default).
	if got := MapStatus("namespace.newthing"); got != http.StatusInternalServerError {
		t.Errorf("MapStatus unknown: want 500, got %d", got)
	}
}

// TestCodeConstants_Stable pins the wire format strings so accidental edits
// break loudly here rather than at the frontend.
func TestCodeConstants_Stable(t *testing.T) {
	cases := map[string]string{
		CodeAuthLoginFail:        "auth.login.fail",
		CodeAuthTokenExpired:     "auth.token.expired",
		CodeAuthAPIKeyRevoked:    "auth.apikey.revoked",
		CodeAuthPermissionDenied: "auth.permission_denied",
		CodeRBACPermissionDenied: "rbac.permission_denied",
		CodeRBACRoleMismatch:     "rbac.role_mismatch",
		CodeDocNotFound:          "doc.not_found",
		CodeDocVersionConflict:   "doc.version_conflict",
		CodeDocCodeMalformed:     "doc.code_malformed",
		CodeBindingNotFound:      "binding.not_found",
		CodeBindingDuplicate:     "binding.duplicate",
		CodeSyncPartialFail:      "sync.partial_fail",
		CodeSyncLocked:           "sync.locked",
		CodeSyncCASUnresolved:    "sync.cas_unresolved",
		CodeSearchUpstreamTimeout: "search.upstream.timeout",
		CodeSearchEmpty:          "search.empty",
		CodeSAGRuntimeUnavailable: "sag.runtime.unavailable",
		CodeInternalError:        "internal.error",
		CodeInternalDBUnavailable: "internal.db_unavailable",
		CodeInternalTimeout:      "internal.timeout",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant drift: want %q, got %q", want, got)
		}
	}
}

// TestWriteJSON_Success exercises the Gin helper for the happy path.
func TestWriteJSON_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)

	WriteJSON(c, http.StatusOK, Ok(map[string]any{"v": 1}, "rid-1"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("body missing success:true: %s", body)
	}
	if !strings.Contains(body, `"request_id":"rid-1"`) {
		t.Fatalf("body missing request_id: %s", body)
	}
	if !strings.Contains(body, `"v":1`) {
		t.Fatalf("body missing data.v: %s", body)
	}
}

// TestWriteJSON_Error covers the failure path that AC-B7 measures.
func TestWriteJSON_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)

	WriteJSON(c, http.StatusUnauthorized,
		New(CodeAuthLoginFail, "wrong password", "rid-err"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: want 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"success":false`,
		`"error":{"code":"auth.login.fail","message":"wrong password"}`,
		`"data":null`,
		`"request_id":"rid-err"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

// TestEnvelope_WithMeta_Frozen guards that meta survives only via WithMeta,
// never by editing frozen fields.
func TestEnvelope_WithMeta_Frozen(t *testing.T) {
	in := New(CodeDocVersionConflict, "stale", "rid")
	out := WithMeta(in, map[string]any{"base_version": int64(7)})
	if out.Meta == nil || out.Meta["base_version"] == nil {
		t.Fatalf("meta not attached: %+v", out)
	}
	// Original must be untouched (immutability by copy).
	if in.Meta != nil {
		t.Fatalf("WithMeta must copy, original mutated: %+v", in.Meta)
	}
}

// TestFromError_CodedError verifies the translator reads Code()/Status().
func TestFromError_CodedError(t *testing.T) {
	err := &fakeCoded{code: CodeDocVersionConflict, status: http.StatusConflict, msg: "stale"}
	env := FromError(err, "rid-x")
	if env.Success {
		t.Fatalf("FromError should produce success=false")
	}
	if env.Error.Code != CodeDocVersionConflict {
		t.Fatalf("error.code: %q", env.Error.Code)
	}
	if env.ExitCode == 0 {
		t.Fatalf("exit_code must be non-zero")
	}
}

// TestFromError_Unknown ensures unknown errors collapse to internal.error.
func TestFromError_Unknown(t *testing.T) {
	env := FromError(stringErr("boom"), "rid-y")
	if env.Error == nil || env.Error.Code != CodeInternalError {
		t.Fatalf("unknown error must collapse to %s, got %+v", CodeInternalError, env.Error)
	}
}

// --- helpers used by the tests above ---

type fakeCoded struct {
	code, msg string
	status    int
}

func (e *fakeCoded) Error() string { return e.msg }
func (e *fakeCoded) Code() string  { return e.code }
func (e *fakeCoded) Status() int   { return e.status }

type stringErr string

func (e stringErr) Error() string { return string(e) }
