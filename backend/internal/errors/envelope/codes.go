// Package envelope owns the frozen v1.0 envelope contract and the canonical
// HTTP-status mapping table for namespace.subname error codes.
//
// The field set on Envelope is frozen (see ../spec/backend/error-handling.md
// and design.md §3.1). Adding fields is a breaking change; new extension goes
// in the optional `meta` sub-object.
package envelope

import "net/http"

// Code is a stable wire-format error identifier ("namespace.subname").
//
// Naming rules (frozen):
//   - Lowercase ASCII, dot-segmented.
//   - No spaces, no underscores.
//   - One frozen subname per concept; don't overload.
//
// Codes are constants on this package — search-and-replace is the wire change.
type Code = string

// Namespace subname constants. Each is owned by exactly one package per
// ../spec/backend/error-handling.md §"Error Code Namespaces". Adding a new
// constant in the same namespace must update MapStatus() below.
const (
	// auth.* — authentication failures (HTTP 401 / 403).
	CodeAuthLoginFail        Code = "auth.login.fail"        // 401 — bad credentials
	CodeAuthTokenExpired     Code = "auth.token.expired"     // 401 — JWT expired
	CodeAuthAPIKeyRevoked    Code = "auth.apikey.revoked"    // 401 — API key disabled/deleted
	CodeAuthPermissionDenied Code = "auth.permission_denied" // 403 — authn ok, rbac denied at auth layer

	// rbac.* — role-based access control (HTTP 403).
	CodeRBACPermissionDenied Code = "rbac.permission_denied" // 403 — role lacks (resource, action)
	CodeRBACRoleMismatch     Code = "rbac.role_mismatch"     // 403 — caller role ≠ required role

	// doc.* — document lifecycle (HTTP 404 / 409 / 422).
	CodeDocNotFound        Code = "doc.not_found"        // 404
	CodeDocVersionConflict Code = "doc.version_conflict" // 409 — CAS base_version stale
	CodeDocCodeMalformed   Code = "doc.code_malformed"   // 422 — docCode fails 8-char base62

	// binding.* — binding lifecycle (HTTP 400 / 404 / 409).
	CodeBindingNotFound  Code = "binding.not_found"  // 404
	CodeBindingDuplicate Code = "binding.duplicate" // 409

	// sync.* — pull/push orchestration (HTTP 207 / 409 / 423).
	CodeSyncPartialFail   Code = "sync.partial_fail"   // 207 — multi-status (pull)
	CodeSyncLocked        Code = "sync.locked"         // 423 — file lock held
	CodeSyncCASUnresolved Code = "sync.cas_unresolved" // 409 — push exit 24

	// search.* — retrieval layer (HTTP 502 / 504). Filled in M2.
	CodeSearchUpstreamTimeout Code = "search.upstream.timeout" // 504
	CodeSearchEmpty           Code = "search.empty"            // 200 (returned as a code, not a transport failure)

	// sag.* — SAG agent runtime (HTTP 502 / 504). Filled in M2.
	CodeSAGRuntimeUnavailable Code = "sag.runtime.unavailable" // 502

	// internal.* — server core failures (HTTP 500).
	CodeInternalError         Code = "internal.error"          // 500 — generic; never leak detail
	CodeInternalDBUnavailable Code = "internal.db_unavailable" // 503 — DB pool exhausted
	CodeInternalTimeout       Code = "internal.timeout"        // 504 — context.DeadlineExceeded
)

// MapStatus returns the canonical HTTP status for a code. Unknown codes map
// to 500 (fail-closed default). Adding a new code MUST add a row here in the
// same commit.
func MapStatus(code Code) int {
	switch code {
	case CodeAuthLoginFail, CodeAuthTokenExpired, CodeAuthAPIKeyRevoked:
		return http.StatusUnauthorized
	case CodeAuthPermissionDenied, CodeRBACPermissionDenied, CodeRBACRoleMismatch:
		return http.StatusForbidden
	case CodeDocNotFound, CodeBindingNotFound:
		return http.StatusNotFound
	case CodeDocVersionConflict, CodeBindingDuplicate, CodeSyncCASUnresolved:
		return http.StatusConflict
	case CodeDocCodeMalformed:
		return http.StatusUnprocessableEntity
	case CodeSyncPartialFail:
		return http.StatusMultiStatus
	case CodeSyncLocked:
		return http.StatusLocked
	case CodeSearchUpstreamTimeout:
		return http.StatusGatewayTimeout
	case CodeSAGRuntimeUnavailable:
		return http.StatusBadGateway
	case CodeInternalDBUnavailable:
		return http.StatusServiceUnavailable
	case CodeInternalTimeout:
		return http.StatusGatewayTimeout
	case CodeInternalError, CodeSearchEmpty:
		// search.empty carries an HTTP 200 (the query worked, no hits) — the
		// status mapping returns 200 so the wrapper WriteJSON uses the right
		// transport code. Handlers MUST inspect env.Success when semantics matter.
		if code == CodeSearchEmpty {
			return http.StatusOK
		}
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
