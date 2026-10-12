// Package rbac owns the AI-KBS 4-role authorization matrix and the
// Gin middleware that enforces it.
//
// Background
// ---------
// AI-KBS is a fork of WeKnora. WeKnora already ships an RBAC layer in
// internal/middleware/rbac.go that uses a numeric TenantRole ladder
// (Owner > Admin > Contributor > Viewer) and writes 403 with a hand-
// rolled gin.H body. That layer stays in place for the routes that
// pre-date M1 (the WeKnora KB / agent / tenant endpoints).
//
// The matrix in this package is the new authorization contract for the
// AI-KBS surface area: workspace + document + binding + sag + search.
// It is (role, resource, action) → bool by design (see
// .trellis/tasks/10-02-ai-kbs-mvp/design.md §2.7 and
// .trellis/tasks/10-09-M1-backend-cli/design.md §2) so the decision
// can be reasoned about without consulting the middleware code.
//
// Errors
// ------
// On rejection, RequireAction / RequireRole write the frozen envelope
// v1.0 response (envelope.CodeRBACPermissionDenied → 403) so the
// error.code on the wire is uniform across auth + rbac. We do NOT
// take a dependency on the WeKnora middleware's hand-rolled body —
// every new endpoint declared in M1 uses this package, and we want
// every rbac rejection to be envelope-shaped.
package rbac

import (
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
)

// Role is the 4-level RBAC role used by the AI-KBS matrix.
//
// We intentionally re-declare the four role constants here rather than
// aliasing types.TenantRole so the matrix package can be unit-tested
// without dragging in the rest of the types tree. The string values
// match types.TenantRole (also "owner" / "admin" / "contributor" /
// "viewer") so callers may freely convert in either direction:
//
//	role := rbac.Role(types.TenantRoleFromContext(ctx))
//	if role == rbac.RoleViewer { ... }
type Role string

const (
	// RoleOwner has full authority over every resource and action in
	// the matrix. Owner-only operations (e.g. workspace.delete) require
	// exactly this role.
	RoleOwner Role = "owner"
	// RoleAdmin can do everything Owner can do, except for owner-only
	// operations like workspace.delete. Admin is the "manage the
	// tenant" role.
	RoleAdmin Role = "admin"
	// RoleContributor can read everything, can create new resources,
	// can update resources they own (out of scope for the matrix; the
	// ownership dimension is enforced at the handler level), and can
	// run / invoke agents.
	RoleContributor Role = "contributor"
	// RoleViewer can only read resources and run agents. No mutations.
	RoleViewer Role = "viewer"
)

// allRoles is the canonical iteration order used by tests and matrix
// enumeration. Highest privilege first so test failure messages read
// "owner must allow X" rather than "viewer must allow X" when the
// privilege ladder is the point.
var allRoles = []Role{RoleOwner, RoleAdmin, RoleContributor, RoleViewer}

// IsValid reports whether r is one of the four defined roles.
func (r Role) IsValid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleContributor, RoleViewer:
		return true
	default:
		return false
	}
}

// String makes Role satisfy fmt.Stringer; used by logger calls and
// the rbac audit path so the role name is the wire-visible string.
func (r Role) String() string { return string(r) }

// Resource is a categorical grouping of operations the matrix
// reasons about. Resources are coarse on purpose (workspace, document,
// binding, sag, search) — the matrix is a coarse-grained gate, not
// a row-level ACL. Row-level checks (e.g. "this document belongs to
// this workspace") happen downstream of the middleware.
type Resource string

const (
	ResourceWorkspace Resource = "workspace"
	ResourceDocument  Resource = "document"
	ResourceBinding   Resource = "binding"
	ResourceSAG       Resource = "sag"
	ResourceSearch    Resource = "search"
)

// allResources mirrors the constants above; used by tests so the test
// list never drifts from the source of truth.
var allResources = []Resource{
	ResourceWorkspace,
	ResourceDocument,
	ResourceBinding,
	ResourceSAG,
	ResourceSearch,
}

// Action is the verb being attempted against a Resource. Together
// (Resource, Action) fully identifies a single matrix cell.
type Action string

const (
	// ActionRead:    GET / list / fetch the resource.
	ActionRead Action = "read"
	// ActionWrite:   PUT / PATCH — mutate an existing resource in place.
	ActionWrite Action = "write"
	// ActionDelete:  DELETE — remove a resource.
	ActionDelete Action = "delete"
	// ActionCreate:  POST — bring a new resource into existence.
	ActionCreate Action = "create"
	// ActionInvoke:  Run a side-effecting operation against an existing
	// resource (e.g. sag.invoke). Distinct from "execute" on search
	// because the verb "invoke" was frozen in design.md §4.3.
	ActionInvoke Action = "invoke"
	// ActionConfig:  Update configuration of a resource (e.g. sag.config
	// — change the agent's prompt / tools without changing the agent
	// itself).
	ActionConfig Action = "config"
	// ActionExecute: Generic run verb for read-only side effects such
	// as search.execute (run a query).
	ActionExecute Action = "execute"
)

// allActions mirrors the constants above.
var allActions = []Action{
	ActionRead,
	ActionWrite,
	ActionDelete,
	ActionCreate,
	ActionInvoke,
	ActionConfig,
	ActionExecute,
}

// spec lists the 10 (resource, action) pairs the matrix covers. The
// contract (parent implement.md §T1.6 / AC-B6) demands 10 entries;
// this slice is the single source of truth so neither tests nor docs
// drift from the matrix implementation.
//
// Naming: "resource.action" is a stable wire identifier — middleware
// rejections surface it in the rbac.* envelope meta so operators can
// grep for "rbac denied: document.delete".
var spec = []struct {
	Resource Resource
	Action   Action
}{
	{ResourceWorkspace, ActionRead},   // 1 — workspace.read
	{ResourceWorkspace, ActionWrite},  // 2 — workspace.write
	{ResourceWorkspace, ActionDelete}, // 3 — workspace.delete (owner only)
	{ResourceDocument, ActionRead},   // 4 — document.read
	{ResourceDocument, ActionWrite},  // 5 — document.write
	{ResourceDocument, ActionDelete}, // 6 — document.delete
	{ResourceBinding, ActionCreate},   // 7 — binding.create
	{ResourceBinding, ActionDelete},  // 8 — binding.delete
	{ResourceSAG, ActionInvoke},      // 9 — sag.invoke
	{ResourceSAG, ActionConfig},      // 10 — sag.config
	{ResourceSearch, ActionExecute},  // 11 — search.execute
}

// SpecEntry describes one row of the matrix; the public form of the
// `spec` table. Exposed so the matrix_test.go can iterate without
// re-typing the (resource, action) list.
type SpecEntry struct {
	Resource Resource
	Action   Action
}

// Spec returns a copy of the (resource, action) list the matrix
// covers. Callers MUST NOT mutate the returned slice. Tests iterate
// this list to drive the 4 × N case grid.
func Spec() []SpecEntry {
	out := make([]SpecEntry, 0, len(spec))
	for _, e := range spec {
		out = append(out, SpecEntry{Resource: e.Resource, Action: e.Action})
	}
	return out
}

// Matrix returns whether `role` is allowed to perform `action` against
// `resource`. This is the single source of truth for the 4-level RBAC
// decision. Unknown roles / resources / actions always return false
// (fail-closed).
//
// The matrix (in spec order):
//
//	role         ws.read  ws.write  ws.delete  doc.read  doc.write  doc.delete  bind.create  bind.delete  sag.invoke  sag.config  search.execute
//	owner        yes      yes       yes        yes       yes        yes         yes          yes          yes         yes         yes
//	admin        yes      yes       no         yes       yes        yes         yes          yes          yes         yes         yes
//	contributor  yes      no        no         yes       yes        no          yes          no           yes         no          yes
//	viewer       yes      no        no         yes       no         no          no           no           yes         no          yes
//
// Rationale (per design.md §2.7 / T1.6 design contract):
//   - owner:   full authority. ws.delete is the only one we list
//     explicitly; everything else flows from the privilege ladder.
//   - admin:   full except owner-only operations (ws.delete). Owns
//     tenant configuration so sag.config and binding lifecycle are
//     included.
//   - contributor: read everything; create new resources (bind.create,
//     sag.invoke to start an agent); can mutate content (doc.write)
//     but not delete or reconfigure (doc.delete, sag.config).
//   - viewer:   read everything; can still invoke agents and run
//     search (these are read-shaped from the caller's POV).
func Matrix(role Role, resource Resource, action Action) bool {
	if !role.IsValid() {
		return false
	}
	if !isKnownResourceAction(resource, action) {
		return false
	}
	switch role {
	case RoleOwner:
		return true
	case RoleAdmin:
		// Owner-only operation: workspace.delete.
		return !(resource == ResourceWorkspace && action == ActionDelete)
	case RoleContributor:
		switch resource {
		case ResourceWorkspace:
			// workspace.write is admin+ only (e.g. rename / change
			// configuration); contributor may only read the workspace.
			return action == ActionRead
		case ResourceDocument:
			// contributor can read + write docs but not delete.
			return action == ActionRead || action == ActionWrite
		case ResourceBinding:
			// contributor can create new bindings but not delete them
			// (deletion is a tenant-management action).
			return action == ActionCreate
		case ResourceSAG:
			// contributor can invoke an agent; reconfiguring the agent
			// itself is admin+.
			return action == ActionInvoke
		case ResourceSearch:
			// search.execute is a read-shaped action; viewer + above.
			return action == ActionExecute
		}
	case RoleViewer:
		switch resource {
		case ResourceWorkspace, ResourceDocument:
			return action == ActionRead
		case ResourceSAG:
			// viewer can run an agent (this is the headline feature
			// for a read-only persona).
			return action == ActionInvoke
		case ResourceSearch:
			// viewer can run search queries.
			return action == ActionExecute
		case ResourceBinding:
			// viewer has no binding lifecycle authority.
			return false
		}
	}
	return false
}

// HasAtLeast reports whether `have` is at or above `min` on the
// privilege ladder. It is a pure ordering check; it does NOT consult
// the matrix. Use it when a caller only has a role floor (no
// resource / action pair) — e.g. "this endpoint requires admin or
// better" — and pair it with Matrix() for per-(resource, action)
// checks.
//
// The function name mirrors the tenant role ladder's HasPermission
// method so the two read similarly at the call site:
//
//	if !rbac.HasAtLeast(role, rbac.RoleAdmin) {
//	    return rbac.PermissionDenied
//	}
func HasAtLeast(have, min Role) bool {
	return roleAtLeast(have, min)
}

// roleAtLeast reports whether `have` is at or above `need` on the
// privilege ladder. Mirrors types.TenantRole.HasPermission without
// pulling the types tree into this package's public API.
func roleAtLeast(have, need Role) bool {
	return roleLevel(have) >= roleLevel(need)
}

// roleLevel returns the privilege level of r (higher = more powerful).
// Unknown roles return 0, which is strictly less than any defined
// role — i.e. fail-closed.
func roleLevel(r Role) int {
	switch r {
	case RoleOwner:
		return 40
	case RoleAdmin:
		return 30
	case RoleContributor:
		return 20
	case RoleViewer:
		return 10
	default:
		return 0
	}
}

// isKnownResourceAction reports whether (resource, action) is one of
// the pairs the matrix declares. Unknown pairs return false from
// Matrix() so a future (resource, action) addition that forgets to
// update the matrix is rejected at the door rather than silently
// permitted.
func isKnownResourceAction(resource Resource, action Action) bool {
	for _, e := range spec {
		if e.Resource == resource && e.Action == action {
			return true
		}
	}
	return false
}

// RoleFromTenantRole converts a types.TenantRole into the rbac.Role
// used by this matrix. Returns (RoleViewer, false) when the input is
// not a recognised role — callers can treat the bool as "valid
// input"; the role value is the safe default.
func RoleFromTenantRole(t types.TenantRole) (Role, bool) {
	switch t {
	case types.TenantRoleOwner:
		return RoleOwner, true
	case types.TenantRoleAdmin:
		return RoleAdmin, true
	case types.TenantRoleContributor:
		return RoleContributor, true
	case types.TenantRoleViewer:
		return RoleViewer, true
	default:
		return RoleViewer, false
	}
}
