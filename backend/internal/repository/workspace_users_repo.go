package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableWorkspaceUsers is the SQL table backing WorkspaceUserRepository.
const TableWorkspaceUsers = "workspace_users"

// WorkspaceUserRepository persists the workspace ↔ user role bindings.
type WorkspaceUserRepository interface {
	Create(ctx context.Context, wu *domain.WorkspaceUser) error
	Get(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceUser, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WorkspaceUser, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.WorkspaceUser, error)
	UpdateRole(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) error
	Delete(ctx context.Context, workspaceID, userID string) error
}

type workspaceUserRepo struct {
	db DBTX
}

// NewWorkspaceUserRepository wires a WorkspaceUserRepository to a DBTX
// (pool or tx).
func NewWorkspaceUserRepository(db *pgxpool.Pool) WorkspaceUserRepository {
	return &workspaceUserRepo{db: db}
}

func (r *workspaceUserRepo) Create(ctx context.Context, wu *domain.WorkspaceUser) error {
	if wu == nil {
		return errors.New("repository: workspace user is nil")
	}
	const q = `INSERT INTO workspace_users (workspace_id, user_id, role, invited_by, joined_at)
	           VALUES ($1, $2, $3, $4, NOW())`
	_, err := r.db.Exec(ctx, q, wu.WorkspaceID, wu.UserID, string(wu.Role), nullable(wu.InvitedBy))
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("workspace_user ws=%s user=%s: %w", wu.WorkspaceID, wu.UserID, mapped)
		}
		return fmt.Errorf("workspace_user.Create ws=%s: %w", wu.WorkspaceID, err)
	}
	return nil
}

func (r *workspaceUserRepo) Get(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceUser, error) {
	const q = `SELECT workspace_id, user_id, role, invited_by, joined_at
	           FROM workspace_users WHERE workspace_id = $1 AND user_id = $2`
	wu, err := scanWorkspaceUser(r.db.QueryRow(ctx, q, workspaceID, userID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("workspace_user.Get ws=%s user=%s: %w", workspaceID, userID, err)
	}
	return wu, nil
}

func (r *workspaceUserRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WorkspaceUser, error) {
	const q = `SELECT workspace_id, user_id, role, invited_by, joined_at
	           FROM workspace_users WHERE workspace_id = $1 ORDER BY joined_at ASC`
	rows, err := r.db.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace_user.ListByWorkspace ws=%s: %w", workspaceID, err)
	}
	defer rows.Close()
	var out []*domain.WorkspaceUser
	for rows.Next() {
		wu, err := scanWorkspaceUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workspace_user.ListByWorkspace rows.Err: %w", err)
	}
	return out, nil
}

func (r *workspaceUserRepo) ListByUser(ctx context.Context, userID string) ([]*domain.WorkspaceUser, error) {
	const q = `SELECT workspace_id, user_id, role, invited_by, joined_at
	           FROM workspace_users WHERE user_id = $1 ORDER BY joined_at ASC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("workspace_user.ListByUser user=%s: %w", userID, err)
	}
	defer rows.Close()
	var out []*domain.WorkspaceUser
	for rows.Next() {
		wu, err := scanWorkspaceUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workspace_user.ListByUser rows.Err: %w", err)
	}
	return out, nil
}

func (r *workspaceUserRepo) UpdateRole(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) error {
	const q = `UPDATE workspace_users SET role = $3 WHERE workspace_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, workspaceID, userID, string(role))
	if err != nil {
		return fmt.Errorf("workspace_user.UpdateRole ws=%s user=%s: %w", workspaceID, userID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *workspaceUserRepo) Delete(ctx context.Context, workspaceID, userID string) error {
	const q = `DELETE FROM workspace_users WHERE workspace_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("workspace_user.Delete ws=%s user=%s: %w", workspaceID, userID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanWorkspaceUser(row pgx.Row) (*domain.WorkspaceUser, error) {
	var wu domain.WorkspaceUser
	var role string
	var invitedBy *string
	err := row.Scan(&wu.WorkspaceID, &wu.UserID, &role, &invitedBy, &wu.JoinedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	wu.Role = domain.WorkspaceRole(role)
	if invitedBy != nil {
		wu.InvitedBy = *invitedBy
	}
	return &wu, nil
}