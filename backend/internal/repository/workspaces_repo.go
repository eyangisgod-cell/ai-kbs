package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableWorkspaces is the SQL table name backing WorkspaceRepository.
// Kept as a package-level constant so a future schema rename is a
// single edit.
const TableWorkspaces = "workspaces"

// WorkspaceRepository persists workspaces.
type WorkspaceRepository interface {
	Create(ctx context.Context, w *domain.Workspace) error
	Get(ctx context.Context, id string) (*domain.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error)
	List(ctx context.Context, ownerID string) ([]*domain.Workspace, error)
	Update(ctx context.Context, w *domain.Workspace) error
	SoftDelete(ctx context.Context, id string) error
}

type workspaceRepo struct {
	db DBTX
}

// NewWorkspaceRepository wires a WorkspaceRepository to a DBTX (pool or
// tx).
func NewWorkspaceRepository(db *pgxpool.Pool) WorkspaceRepository {
	return &workspaceRepo{db: db}
}

// newWorkspaceRepositoryWithTx builds a repo bound to a transactional
// DBTX. Used internally by tests; production code composes via
// NewRepositories.
func newWorkspaceRepositoryWithTx(db DBTX) WorkspaceRepository {
	return &workspaceRepo{db: db}
}

func (r *workspaceRepo) Create(ctx context.Context, w *domain.Workspace) error {
	if w == nil {
		return errors.New("repository: workspace is nil")
	}
	const q = `INSERT INTO workspaces (id, name, slug, owner_id, created_at, updated_at, deleted_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, NOW(), NOW(), NULL)`
	_, err := r.db.Exec(ctx, q, w.ID, w.Name, w.Slug, w.OwnerID)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("workspace slug=%q: %w", w.Slug, mapped)
		}
		return fmt.Errorf("workspace.Create slug=%q: %w", w.Slug, err)
	}
	return nil
}

func (r *workspaceRepo) Get(ctx context.Context, id string) (*domain.Workspace, error) {
	const q = `SELECT id, name, slug, owner_id, created_at, updated_at, deleted_at
	           FROM workspaces WHERE id = $1 AND deleted_at IS NULL`
	w, err := scanWorkspace(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("workspace.Get id=%s: %w", id, err)
	}
	return w, nil
}

func (r *workspaceRepo) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	const q = `SELECT id, name, slug, owner_id, created_at, updated_at, deleted_at
	           FROM workspaces WHERE slug = $1 AND deleted_at IS NULL`
	w, err := scanWorkspace(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("workspace.GetBySlug slug=%s: %w", slug, err)
	}
	return w, nil
}

func (r *workspaceRepo) List(ctx context.Context, ownerID string) ([]*domain.Workspace, error) {
	const q = `SELECT id, name, slug, owner_id, created_at, updated_at, deleted_at
	           FROM workspaces WHERE owner_id = $1 AND deleted_at IS NULL
	           ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("workspace.List owner=%s: %w", ownerID, err)
	}
	defer rows.Close()
	var out []*domain.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workspace.List rows.Err: %w", err)
	}
	return out, nil
}

func (r *workspaceRepo) Update(ctx context.Context, w *domain.Workspace) error {
	if w == nil {
		return errors.New("repository: workspace is nil")
	}
	const q = `UPDATE workspaces SET name = $2, slug = $3, updated_at = NOW()
	           WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, w.ID, w.Name, w.Slug)
	if err != nil {
		return fmt.Errorf("workspace.Update id=%s: %w", w.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *workspaceRepo) SoftDelete(ctx context.Context, id string) error {
	const q = `UPDATE workspaces SET deleted_at = NOW(), updated_at = NOW()
	           WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("workspace.SoftDelete id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanWorkspace reads a workspace row from either pgx.Row or pgx.Rows.
func scanWorkspace(row pgx.Row) (*domain.Workspace, error) {
	var w domain.Workspace
	var deletedAt *time.Time
	err := row.Scan(&w.ID, &w.Name, &w.Slug, &w.OwnerID, &w.CreatedAt, &w.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if deletedAt != nil {
		w.DeletedAt = deletedAt
	}
	return &w, nil
}