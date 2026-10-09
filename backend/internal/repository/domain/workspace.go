package domain

import "time"

// Workspace is the AI-KBS workspace aggregate root. Maps 1:1 to the
// `workspaces` table (migration 000079).
type Workspace struct {
	ID        string
	Name      string
	Slug      string
	OwnerID   string
	CreatedAt time.Time
	UpdatedAt time.Time
	// DeletedAt is non-nil for soft-deleted workspaces. Repositories
	// MUST filter soft-deleted rows out of Get/List unless the caller
	// explicitly asks for the trash.
	DeletedAt *time.Time
}

// IsDeleted reports whether the workspace has been soft-deleted.
func (w *Workspace) IsDeleted() bool { return w != nil && w.DeletedAt != nil }