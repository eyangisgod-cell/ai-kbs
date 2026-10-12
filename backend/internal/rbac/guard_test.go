package rbac

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
)

// guardHarness builds a tiny gin engine that mirrors the auth
// middleware's contract: a per-test setup function sets the caller's
// role / API-key flag on the request context and (if the test
// cares) on the gin.Keys map. Returning the engine (rather than
// the recorder) keeps the harness builder composable: tests that
// need to register more than one route can append to the engine
// before serving.
func guardHarness(t *testing.T, role types.TenantRole, machinePrincipal bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.UserIDContextKey, "test-user")
		c.Request = c.Request.WithContext(ctx)
		if machinePrincipal {
			c.Set(apiKeyCapabilitiesContextKey, []string{"read"})
		}
		c.Next()
	})
	return r
}

// runGuard wires a single guarded route into the harness and serves
// a single request. Returns the recorder so the caller can assert on
// status + body.
func runGuard(t *testing.T, role types.TenantRole, machine bool, mw gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := guardHarness(t, role, machine)
	r.GET("/protected", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	return w
}

// TestRequireAction_OwnerAllowedEverywhere proves that the owner
// role clears every (resource, action) gate the matrix declares.
// One assertion per Spec row = 11 cases.
func TestRequireAction_OwnerAllowedEverywhere(t *testing.T) {
	for _, entry := range Spec() {
		w := runGuard(t, types.TenantRoleOwner, false, RequireAction(entry.Resource, entry.Action))
		if w.Code != http.StatusOK {
			t.Errorf("owner should clear %s.%s, got %d", entry.Resource, entry.Action, w.Code)
		}
	}
}

// TestRequireAction_ViewerAllowedReadAndRun covers the viewer
// persona's allowed (resource, action) cells: read on workspace +
// document, invoke on sag, execute on search. Everything else must
// reject with envelope.CodeRBACPermissionDenied (403).
func TestRequireAction_ViewerAllowedReadAndRun(t *testing.T) {
	allowed := []struct {
		Resource Resource
		Action   Action
	}{
		{ResourceWorkspace, ActionRead},
		{ResourceDocument, ActionRead},
		{ResourceSAG, ActionInvoke},
		{ResourceSearch, ActionExecute},
	}
	for _, c := range allowed {
		w := runGuard(t, types.TenantRoleViewer, false, RequireAction(c.Resource, c.Action))
		if w.Code != http.StatusOK {
			t.Errorf("viewer should be allowed %s.%s, got %d", c.Resource, c.Action, w.Code)
		}
	}
}

// TestRequireAction_ViewerDeniedEverywhereElse checks every (resource,
// action) cell the matrix rejects for viewer, including the
// (resource, action) pairs that owner / admin / contributor can do.
// This is the negative half of the viewer persona.
func TestRequireAction_ViewerDeniedEverywhereElse(t *testing.T) {
	denied := []struct {
		Resource Resource
		Action   Action
	}{
		{ResourceWorkspace, ActionWrite},
		{ResourceWorkspace, ActionDelete},
		{ResourceDocument, ActionWrite},
		{ResourceDocument, ActionDelete},
		{ResourceBinding, ActionCreate},
		{ResourceBinding, ActionDelete},
		{ResourceSAG, ActionConfig},
	}
	for _, c := range denied {
		w := runGuard(t, types.TenantRoleViewer, false, RequireAction(c.Resource, c.Action))
		if w.Code != http.StatusForbidden {
			t.Errorf("viewer must be denied %s.%s, got %d", c.Resource, c.Action, w.Code)
		}
		assertEnvelopeRejection(t, w)
	}
}

// TestRequireAction_AdminDeniedWorkspaceDelete nails down the only
// admin / (resource, action) rejection. Admin is "everything except
// owner-only" — workspace.delete is the owner-only operation in the
// design contract.
func TestRequireAction_AdminDeniedWorkspaceDelete(t *testing.T) {
	w := runGuard(t, types.TenantRoleAdmin, false, RequireAction(ResourceWorkspace, ActionDelete))
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin must be denied workspace.delete, got %d", w.Code)
	}
	assertEnvelopeRejection(t, w)
}

// TestRequireAction_AdminAllowedEverywhereElse checks every other
// (resource, action) cell against the admin role. After
// AdminDeniedWorkspaceDelete above, this is the positive half of
// the admin persona.
func TestRequireAction_AdminAllowedEverywhereElse(t *testing.T) {
	allowed := []struct {
		Resource Resource
		Action   Action
	}{
		{ResourceWorkspace, ActionRead},
		{ResourceWorkspace, ActionWrite},
		{ResourceDocument, ActionRead},
		{ResourceDocument, ActionWrite},
		{ResourceDocument, ActionDelete},
		{ResourceBinding, ActionCreate},
		{ResourceBinding, ActionDelete},
		{ResourceSAG, ActionInvoke},
		{ResourceSAG, ActionConfig},
		{ResourceSearch, ActionExecute},
	}
	for _, c := range allowed {
		w := runGuard(t, types.TenantRoleAdmin, false, RequireAction(c.Resource, c.Action))
		if w.Code != http.StatusOK {
			t.Errorf("admin should be allowed %s.%s, got %d", c.Resource, c.Action, w.Code)
		}
	}
}

// TestRequireAction_ContributorAllowed covers the contributor
// persona's allowed cells: read everywhere, write on document,
// create binding, invoke sag, execute search.
func TestRequireAction_ContributorAllowed(t *testing.T) {
	allowed := []struct {
		Resource Resource
		Action   Action
	}{
		{ResourceWorkspace, ActionRead},
		{ResourceDocument, ActionRead},
		{ResourceDocument, ActionWrite},
		{ResourceBinding, ActionCreate},
		{ResourceSAG, ActionInvoke},
		{ResourceSearch, ActionExecute},
	}
	for _, c := range allowed {
		w := runGuard(t, types.TenantRoleContributor, false, RequireAction(c.Resource, c.Action))
		if w.Code != http.StatusOK {
			t.Errorf("contributor should be allowed %s.%s, got %d", c.Resource, c.Action, w.Code)
		}
	}
}

// TestRequireAction_ContributorDenied covers the contributor
// persona's denied cells: workspace.write (admin+), document.delete,
// binding.delete, sag.config.
func TestRequireAction_ContributorDenied(t *testing.T) {
	denied := []struct {
		Resource Resource
		Action   Action
	}{
		{ResourceWorkspace, ActionWrite},
		{ResourceWorkspace, ActionDelete},
		{ResourceDocument, ActionDelete},
		{ResourceBinding, ActionDelete},
		{ResourceSAG, ActionConfig},
	}
	for _, c := range denied {
		w := runGuard(t, types.TenantRoleContributor, false, RequireAction(c.Resource, c.Action))
		if w.Code != http.StatusForbidden {
			t.Errorf("contributor must be denied %s.%s, got %d", c.Resource, c.Action, w.Code)
		}
		assertEnvelopeRejection(t, w)
	}
}

// TestRequireRole_OwnerAboveAll is the role-only gate equivalent of
// the action matrix: owner must clear every role floor, viewer must
// only clear the viewer floor.
func TestRequireRole_OwnerAboveAll(t *testing.T) {
	for _, min := range []Role{RoleOwner, RoleAdmin, RoleContributor, RoleViewer} {
		w := runGuard(t, types.TenantRoleOwner, false, RequireRole(min))
		if w.Code != http.StatusOK {
			t.Errorf("owner should clear %s floor, got %d", min, w.Code)
		}
	}
}

// TestRequireRole_ViewerOnlyClearsViewerFloor: viewer can only pass
// the viewer floor. Anything stricter (contributor / admin / owner)
// must reject.
func TestRequireRole_ViewerOnlyClearsViewerFloor(t *testing.T) {
	if w := runGuard(t, types.TenantRoleViewer, false, RequireRole(RoleViewer)); w.Code != http.StatusOK {
		t.Errorf("viewer should clear viewer floor, got %d", w.Code)
	}
	for _, min := range []Role{RoleContributor, RoleAdmin, RoleOwner} {
		w := runGuard(t, types.TenantRoleViewer, false, RequireRole(min))
		if w.Code != http.StatusForbidden {
			t.Errorf("viewer must NOT clear %s floor, got %d", min, w.Code)
		}
		assertEnvelopeRejection(t, w)
	}
}

// TestRequireAction_MachinePrincipalShortCircuits asserts the
// API-key short-circuit: the rbac matrix does not apply to machine
// principals. The auth package's APIKeyGate is the authority; the
// rbac guard must not add a second opinion.
func TestRequireAction_MachinePrincipalShortCircuits(t *testing.T) {
	// viewer + machine principal: would normally fail every
	// mutation; with the short-circuit it must pass.
	w := runGuard(t, types.TenantRoleViewer, true, RequireAction(ResourceDocument, ActionDelete))
	if w.Code != http.StatusOK {
		t.Errorf("machine principal must short-circuit the rbac matrix, got %d", w.Code)
	}
}

// TestRequireRole_MachinePrincipalShortCircuits is the role-only
// version of the above: API keys bypass the role floor too.
func TestRequireRole_MachinePrincipalShortCircuits(t *testing.T) {
	w := runGuard(t, types.TenantRoleViewer, true, RequireRole(RoleOwner))
	if w.Code != http.StatusOK {
		t.Errorf("machine principal must short-circuit RequireRole, got %d", w.Code)
	}
}

// TestRejectionBodyIsEnvelope confirms the wire body of a denial
// matches the frozen envelope v1.0 contract: success=false,
// error.code=rbac.permission_denied, exit_code=403, request_id
// echoing X-Request-ID.
func TestRejectionBodyIsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/protected", RequireAction(ResourceWorkspace, ActionDelete), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Request-ID", "req-abc-123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status: want 403, got %d", w.Code)
	}
	var env envelope.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not envelope JSON: %v\nraw: %s", err, w.Body.String())
	}
	if env.Success {
		t.Errorf("envelope.success: want false, got true")
	}
	if env.Error == nil {
		t.Fatalf("envelope.error: want non-nil, got nil")
	}
	if env.Error.Code != envelope.CodeRBACPermissionDenied {
		t.Errorf("envelope.error.code: want %q, got %q", envelope.CodeRBACPermissionDenied, env.Error.Code)
	}
	if env.ExitCode != 403 {
		t.Errorf("envelope.exit_code: want 403, got %d", env.ExitCode)
	}
	if env.RequestID != "req-abc-123" {
		t.Errorf("envelope.request_id: want req-abc-123, got %q", env.RequestID)
	}
	if w.Header().Get("X-Request-ID") != "req-abc-123" {
		t.Errorf("X-Request-ID header: want req-abc-123, got %q", w.Header().Get("X-Request-ID"))
	}
}

// TestRejectionBody_NoRequestIDLeavesRequestIDEmpty checks that the
// envelope's request_id is empty (not synthesised) when the client
// didn't send X-Request-ID. Middleware in the request pipeline
// upstream mints the id; the rbac guard is not the one that does.
func TestRejectionBody_NoRequestIDLeavesRequestIDEmpty(t *testing.T) {
	w := runGuard(t, types.TenantRoleViewer, false, RequireAction(ResourceWorkspace, ActionDelete))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: want 403, got %d", w.Code)
	}
	var env envelope.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not envelope JSON: %v", err)
	}
	if env.RequestID != "" {
		t.Errorf("envelope.request_id: want empty, got %q", env.RequestID)
	}
}

// assertEnvelopeRejection asserts the standard rbac rejection body.
// Used by every "must deny" case so we don't re-type the envelope
// shape per test.
func assertEnvelopeRejection(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var env envelope.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("rejection body is not envelope JSON: %v\nraw: %s", err, w.Body.String())
	}
	if env.Success {
		t.Errorf("rejection envelope.success: want false, got true")
	}
	if env.Error == nil {
		t.Fatal("rejection envelope.error: want non-nil")
	}
	if env.Error.Code != envelope.CodeRBACPermissionDenied {
		t.Errorf("rejection envelope.error.code: want %q, got %q",
			envelope.CodeRBACPermissionDenied, env.Error.Code)
	}
	if env.ExitCode != 403 {
		t.Errorf("rejection envelope.exit_code: want 403, got %d", env.ExitCode)
	}
}
