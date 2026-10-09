package domain

import "time"

// WorkspaceRole enumerates the four RBAC roles from
// `.trellis/spec/backend/error-handling.md` §4 (rbac matrix). Strings
// match what the 000079 migration accepts (`VARCHAR(16)`).
type WorkspaceRole string

const (
	WorkspaceRoleOwner        WorkspaceRole = "owner"
	WorkspaceRoleAdmin        WorkspaceRole = "admin"
	WorkspaceRoleContributor  WorkspaceRole = "contributor"
	WorkspaceRoleViewer       WorkspaceRole = "viewer"
)

// WorkspaceUser is the membership row binding a user to a workspace
// with a role. Maps to the `workspace_users` table.
type WorkspaceUser struct {
	WorkspaceID string
	UserID      string
	Role        WorkspaceRole
	InvitedBy   string
	JoinedAt    time.Time
}