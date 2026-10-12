package rbac

import (
	"net/http"

	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/logger"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
)

// apiKeyCapabilitiesContextKey mirrors the auth package's
// api_key_capabilities key. The guard skips RBAC for machine
// principals (see short-circuit logic below) so we only need this
// to detect that the call came in via API key, not to read the
// capabilities themselves. The auth middleware (T1.5) stores the
// resolved *types.User under the gin key "user"; we deliberately
// do not import that string here because reading the user is the
// role ladder's job, not the guard's.
const apiKeyCapabilitiesContextKey = "api_key_capabilities"

// requestIDHeader is the X-Request-ID header the auth/error envelope
// pipeline honours. We read it (rather than minting a fresh one) so
// the rbac envelope's request_id matches the upstream middleware's
// id and operators can grep a single request across logs.
const requestIDHeader = "X-Request-ID"

// PermissionDenied is the canonical envelope a handler can return
// when it has independently determined (outside the middleware path)
// that the caller lacks authority. Use it from a handler that has
// to consult a request body / db row before deciding:
//
//	if rbac.Matrix(role, rbac.ResourceBinding, rbac.ActionDelete) == false {
//	    envelope.WriteJSON(c, http.StatusForbidden, rbac.PermissionDenied)
//	    return
//	}
//
// The middleware in this file already returns the same shape on
// rejection; this exported value exists for the handler-side path
// so a single envelope template covers both call sites.
var PermissionDenied = envelope.New(envelope.CodeRBACPermissionDenied, "permission denied", "")

// RequireRole returns a Gin middleware that allows the request iff
// the caller's role is at least `min` on the rbac privilege ladder.
// This is the role-only gate (no resource/action semantics) and
// exists as a convenience for endpoints that have a single role
// floor (e.g. "admin only") without a per-(resource, action) check.
//
// On rejection it writes the frozen envelope v1.0 response with
// code = rbac.permission_denied, status = 403, request_id = the
// X-Request-ID header (or empty if absent).
//
// Like all guards in this package, machine principals (API keys)
// short-circuit here: the APIKeyGate / rbac_*.go layer in
// internal/middleware is the source of truth for machine credentials.
// This is intentional — the matrix is a per-tenant ladder that does
// not apply to scoped API keys.
func RequireRole(min Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isMachinePrincipal(c) {
			c.Next()
			return
		}
		role := roleFromGinContext(c)
		if roleAtLeast(role, min) {
			c.Next()
			return
		}
		deny(c, "role "+string(role)+" below required "+string(min))
	}
}

// RequireAction returns a Gin middleware that allows the request
// iff Matrix(role, resource, action) returns true for the caller's
// role. This is the per-(resource, action) gate that the
// (role, resource, action) → bool design contract calls for. Use it
// on routes that need a fine-grained permission, e.g. POST /bindings
// (binding.create) vs DELETE /bindings/:id (binding.delete).
//
// Like RequireRole, machine principals short-circuit.
func RequireAction(resource Resource, action Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isMachinePrincipal(c) {
			c.Next()
			return
		}
		role := roleFromGinContext(c)
		if Matrix(role, resource, action) {
			c.Next()
			return
		}
		deny(c, "role "+string(role)+" denied "+string(resource)+"."+string(action))
	}
}

// roleFromGinContext reads the caller's rbac.Role. The auth
// middleware (T1.5) sets the role under types.TenantRoleContextKey
// using the existing WeKnora TenantRole ladder. We map that to our
// rbac.Role via RoleFromTenantRole; if the conversion is invalid
// (e.g. someone hand-set an unknown role) we fall back to RoleViewer
// — the safe fail-closed default that matches TenantRoleFromContext's
// own behaviour.
func roleFromGinContext(c *gin.Context) Role {
	if c == nil || c.Request == nil {
		return RoleViewer
	}
	t, ok := RoleFromTenantRole(types.TenantRoleFromContext(c.Request.Context()))
	if !ok {
		return RoleViewer
	}
	return t
}

// isMachinePrincipal returns true if the call was authenticated via
// an API key (the auth middleware's "api_key_capabilities" branch).
// The rbac matrix is a per-tenant ladder and does not apply to
// machine principals; the separate apiKeyAuthorizer in
// internal/middleware is the canonical authority.
func isMachinePrincipal(c *gin.Context) bool {
	if c == nil {
		return false
	}
	_, ok := c.Get(apiKeyCapabilitiesContextKey)
	return ok
}

// deny writes the rbac.permission_denied envelope, aborts the gin
// chain, and emits a structured log line. The envelope body is
// frozen; the only wire-visible knobs are:
//   - error.code     = "rbac.permission_denied"  (always)
//   - error.message  = reason (free-form, log-only by convention)
//   - request_id     = X-Request-ID header (so the client + server
//                      can correlate; envelope's RequestID field)
//   - HTTP status    = 403 (from MapStatus(CodeRBACPermissionDenied))
//
// We deliberately do NOT put the (role, resource, action) in the
// wire message — that would let a probing client learn which
// (role, resource, action) cell they tripped. The structured log
// line is server-side only.
func deny(c *gin.Context, reason string) {
	requestID := c.GetHeader(requestIDHeader)
	env := envelope.New(envelope.CodeRBACPermissionDenied, "permission denied", requestID)
	envelope.WriteJSON(c, http.StatusForbidden, env)
	// Structured log: server-side only; includes the failure reason so
	// operators can grep "rbac denied" without scraping the message
	// field. The package logger already wraps the request_id (and
	// trace_id on the OTLP path) via context, so we just hand it
	// the request context as-is.
	logger.Warnf(
		c.Request.Context(),
		"[rbac] %s path=%s",
		reason,
		c.Request.URL.Path,
	)
	c.Abort()
}
