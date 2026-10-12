package rbac

import (
	"testing"

	"github.com/eyangisgod-cell/ai-kbs/internal/types"
)

// matrixCases returns the full (role, resource, action) grid we want
// to assert against. We pin the expected allow/deny per cell by hand
// below so a regression in Matrix() fails loudly. The expected table
// is also the human-readable documentation of the matrix: if you
// change a cell, update the comment in matrix.go and the test in
// the same commit.
//
// Spec coverage: 4 roles × 11 (resource, action) pairs = 44 cases.
// AC-B6 threshold: 40 cases. We exceed the threshold to document the
// viewer/contributor transitions on sag.invoke / search.execute,
// which the design contract calls out explicitly.
var matrixCases = []struct {
	Name     string
	Role     Role
	Resource Resource
	Action   Action
	Allowed  bool
}{
	// --- workspace ---
	{"owner can read workspace", RoleOwner, ResourceWorkspace, ActionRead, true},
	{"owner can write workspace", RoleOwner, ResourceWorkspace, ActionWrite, true},
	{"owner can delete workspace", RoleOwner, ResourceWorkspace, ActionDelete, true},
	{"admin can read workspace", RoleAdmin, ResourceWorkspace, ActionRead, true},
	{"admin can write workspace", RoleAdmin, ResourceWorkspace, ActionWrite, true},
	{"admin CANNOT delete workspace (owner-only)", RoleAdmin, ResourceWorkspace, ActionDelete, false},
	{"contributor can read workspace", RoleContributor, ResourceWorkspace, ActionRead, true},
	{"contributor CANNOT write workspace", RoleContributor, ResourceWorkspace, ActionWrite, false},
	{"contributor CANNOT delete workspace", RoleContributor, ResourceWorkspace, ActionDelete, false},
	{"viewer can read workspace", RoleViewer, ResourceWorkspace, ActionRead, true},
	{"viewer CANNOT write workspace", RoleViewer, ResourceWorkspace, ActionWrite, false},
	{"viewer CANNOT delete workspace", RoleViewer, ResourceWorkspace, ActionDelete, false},

	// --- document ---
	{"owner can read document", RoleOwner, ResourceDocument, ActionRead, true},
	{"owner can write document", RoleOwner, ResourceDocument, ActionWrite, true},
	{"owner can delete document", RoleOwner, ResourceDocument, ActionDelete, true},
	{"admin can read document", RoleAdmin, ResourceDocument, ActionRead, true},
	{"admin can write document", RoleAdmin, ResourceDocument, ActionWrite, true},
	{"admin can delete document", RoleAdmin, ResourceDocument, ActionDelete, true},
	{"contributor can read document", RoleContributor, ResourceDocument, ActionRead, true},
	{"contributor can write document", RoleContributor, ResourceDocument, ActionWrite, true},
	{"contributor CANNOT delete document", RoleContributor, ResourceDocument, ActionDelete, false},
	{"viewer can read document", RoleViewer, ResourceDocument, ActionRead, true},
	{"viewer CANNOT write document", RoleViewer, ResourceDocument, ActionWrite, false},
	{"viewer CANNOT delete document", RoleViewer, ResourceDocument, ActionDelete, false},

	// --- binding ---
	{"owner can create binding", RoleOwner, ResourceBinding, ActionCreate, true},
	{"owner can delete binding", RoleOwner, ResourceBinding, ActionDelete, true},
	{"admin can create binding", RoleAdmin, ResourceBinding, ActionCreate, true},
	{"admin can delete binding", RoleAdmin, ResourceBinding, ActionDelete, true},
	{"contributor can create binding", RoleContributor, ResourceBinding, ActionCreate, true},
	{"contributor CANNOT delete binding", RoleContributor, ResourceBinding, ActionDelete, false},
	{"viewer CANNOT create binding", RoleViewer, ResourceBinding, ActionCreate, false},
	{"viewer CANNOT delete binding", RoleViewer, ResourceBinding, ActionDelete, false},

	// --- sag ---
	{"owner can invoke sag", RoleOwner, ResourceSAG, ActionInvoke, true},
	{"owner can config sag", RoleOwner, ResourceSAG, ActionConfig, true},
	{"admin can invoke sag", RoleAdmin, ResourceSAG, ActionInvoke, true},
	{"admin can config sag", RoleAdmin, ResourceSAG, ActionConfig, true},
	{"contributor can invoke sag", RoleContributor, ResourceSAG, ActionInvoke, true},
	{"contributor CANNOT config sag", RoleContributor, ResourceSAG, ActionConfig, false},
	{"viewer can invoke sag", RoleViewer, ResourceSAG, ActionInvoke, true},
	{"viewer CANNOT config sag", RoleViewer, ResourceSAG, ActionConfig, false},

	// --- search ---
	{"owner can execute search", RoleOwner, ResourceSearch, ActionExecute, true},
	{"admin can execute search", RoleAdmin, ResourceSearch, ActionExecute, true},
	{"contributor can execute search", RoleContributor, ResourceSearch, ActionExecute, true},
	{"viewer can execute search", RoleViewer, ResourceSearch, ActionExecute, true},
}

// TestMatrix_AllCases runs the full 4×N matrix table-driven.
// AC-B6 threshold is 40 cases; this test runs 44.
func TestMatrix_AllCases(t *testing.T) {
	if len(matrixCases) < 40 {
		t.Fatalf("matrixCases covers %d cases, AC-B6 requires >= 40", len(matrixCases))
	}
	for _, tc := range matrixCases {
		t.Run(tc.Name, func(t *testing.T) {
			got := Matrix(tc.Role, tc.Resource, tc.Action)
			if got != tc.Allowed {
				t.Fatalf(
					"Matrix(%s, %s, %s) = %v, want %v",
					tc.Role, tc.Resource, tc.Action, got, tc.Allowed,
				)
			}
		})
	}
}

// TestMatrix_SpecCoversAllRoles ensures Spec() returns a non-empty
// list and every (resource, action) pair is exercised by at least one
// test case. A new pair added to Spec() that isn't tested will fail
// here, so the spec table and the test grid stay in lock-step.
func TestMatrix_SpecCoversAllRoles(t *testing.T) {
	spec := Spec()
	if len(spec) == 0 {
		t.Fatal("Spec() returned empty list")
	}
	// Cross-check: every Spec entry must appear in matrixCases at
	// least once. Iterate by (resource, action) so an unused spec
	// row fails the test.
	for _, entry := range spec {
		found := false
		for _, tc := range matrixCases {
			if tc.Resource == entry.Resource && tc.Action == entry.Action {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Spec entry %s.%s is not covered by any test case", entry.Resource, entry.Action)
		}
	}
	// Sanity: every (resource, action) in matrixCases must appear in
	// Spec(); otherwise a typo'd test would silently pass against an
	// unknown pair.
	for _, tc := range matrixCases {
		found := false
		for _, entry := range spec {
			if tc.Resource == entry.Resource && tc.Action == entry.Action {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("matrixCases row %s.%s is not in Spec()", tc.Resource, tc.Action)
		}
	}
}

// TestMatrix_OwnerAllowAll asserts that the owner role is
// unconditionally allowed for every Spec entry. Owner is the top of
// the ladder; if any (resource, action) cell rejects owner, the
// matrix is wrong.
func TestMatrix_OwnerAllowAll(t *testing.T) {
	for _, entry := range Spec() {
		if !Matrix(RoleOwner, entry.Resource, entry.Action) {
			t.Errorf("owner must be allowed %s.%s", entry.Resource, entry.Action)
		}
	}
}

// TestMatrix_UnknownRoleRejected asserts the fail-closed default for
// an undefined role. A typo'd role string must NOT silently grant
// access; the matrix is the only authority.
func TestMatrix_UnknownRoleRejected(t *testing.T) {
	for _, entry := range Spec() {
		if Matrix(Role("god"), entry.Resource, entry.Action) {
			t.Errorf("unknown role 'god' must NOT be allowed %s.%s", entry.Resource, entry.Action)
		}
	}
}

// TestMatrix_UnknownPairRejected asserts the fail-closed default for
// an unknown (resource, action) pair. If a future endpoint declares
// a new pair, the matrix will reject it until Spec() is updated
// (which is exactly what we want — no silent grants).
func TestMatrix_UnknownPairRejected(t *testing.T) {
	for _, r := range allRoles {
		if Matrix(r, Resource("artifact"), Action("sign")) {
			t.Errorf("unknown pair artifact.sign must NOT be allowed for role %s", r)
		}
	}
}

// TestMatrix_ViewerSagInvokeSearchExecute is the explicit viewer
// transition the design contract calls out: viewer must be able to
// invoke agents and run searches. These are the read-shaped actions
// that give a read-only persona their headline value.
func TestMatrix_ViewerSagInvokeSearchExecute(t *testing.T) {
	if !Matrix(RoleViewer, ResourceSAG, ActionInvoke) {
		t.Error("viewer must be allowed sag.invoke")
	}
	if !Matrix(RoleViewer, ResourceSearch, ActionExecute) {
		t.Error("viewer must be allowed search.execute")
	}
}

// TestRole_Valid ensures the IsValid() contract for roles.
func TestRole_Valid(t *testing.T) {
	for _, r := range []Role{RoleOwner, RoleAdmin, RoleContributor, RoleViewer} {
		if !r.IsValid() {
			t.Errorf("%s should be valid", r)
		}
	}
	for _, r := range []Role{"", "god", "root", "OWNER"} {
		if r.IsValid() {
			t.Errorf("%q should NOT be valid", string(r))
		}
	}
}

// TestRole_String asserts the Stringer hook returns the same
// lowercase string the auth layer expects on the wire. Logger calls
// in the guard use string(role) so the format must stay stable.
func TestRole_String(t *testing.T) {
	cases := []struct {
		r    Role
		want string
	}{
		{RoleOwner, "owner"},
		{RoleAdmin, "admin"},
		{RoleContributor, "contributor"},
		{RoleViewer, "viewer"},
		{Role("custom"), "custom"},
	}
	for _, tc := range cases {
		if got := tc.r.String(); got != tc.want {
			t.Errorf("Role(%q).String() = %q, want %q", string(tc.r), got, tc.want)
		}
	}
}

// TestMatrix_AllResourcesAndActionsCovered walks the spec
// exhaustively to assert that every Spec entry has a non-empty
// resource and action. Catches a typo'd empty constant in
// matrix.go before it silently propagates to the matrix.
func TestMatrix_AllResourcesAndActionsCovered(t *testing.T) {
	for _, entry := range Spec() {
		if string(entry.Resource) == "" {
			t.Errorf("Spec returned empty resource")
		}
		if string(entry.Action) == "" {
			t.Errorf("Spec returned empty action for resource %s", entry.Resource)
		}
	}
}

// TestRoleFromTenantRole covers the conversion from the auth
// package's types.TenantRole (set on the request context by the
// auth middleware) to this package's rbac.Role.
func TestRoleFromTenantRole(t *testing.T) {
	cases := []struct {
		in   types.TenantRole
		want Role
		ok   bool
	}{
		{types.TenantRoleOwner, RoleOwner, true},
		{types.TenantRoleAdmin, RoleAdmin, true},
		{types.TenantRoleContributor, RoleContributor, true},
		{types.TenantRoleViewer, RoleViewer, true},
		{types.TenantRole("alien"), RoleViewer, false},
		{types.TenantRole(""), RoleViewer, false},
	}
	for _, tc := range cases {
		got, ok := RoleFromTenantRole(tc.in)
		if ok != tc.ok {
			t.Errorf("RoleFromTenantRole(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if got != tc.want {
			t.Errorf("RoleFromTenantRole(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// TestHasAtLeast covers the role-ladder ordering check. Owner must
// satisfy any floor; viewer must satisfy only viewer floors. This
// is a pure ordering check; per-(resource, action) authority is
// answered by Matrix() (covered above).
func TestHasAtLeast(t *testing.T) {
	// Owner must be at least admin.
	if !HasAtLeast(RoleOwner, RoleAdmin) {
		t.Error("owner must be at least admin")
	}
	// Viewer must not be at least admin.
	if HasAtLeast(RoleViewer, RoleAdmin) {
		t.Error("viewer must NOT be at least admin")
	}
	// Viewer at viewer should pass.
	if !HasAtLeast(RoleViewer, RoleViewer) {
		t.Error("viewer must be at least viewer")
	}
	// Contributor at admin should fail.
	if HasAtLeast(RoleContributor, RoleAdmin) {
		t.Error("contributor must NOT be at least admin")
	}
	// Admin at admin should pass.
	if !HasAtLeast(RoleAdmin, RoleAdmin) {
		t.Error("admin must be at least admin")
	}
	// Unknown roles are level 0 — must never satisfy any valid floor.
	if HasAtLeast(Role("god"), RoleViewer) {
		t.Error("unknown role must not satisfy viewer floor")
	}
	// Equal-role check is the boundary case — must succeed.
	if !HasAtLeast(RoleContributor, RoleContributor) {
		t.Error("contributor must be at least contributor")
	}
}
