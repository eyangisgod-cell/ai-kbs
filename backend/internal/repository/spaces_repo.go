package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableSpaces is the SQL table backing SpaceRepository.
const TableSpaces = "spaces"

// SpaceRepository persists spaces.
type SpaceRepository interface {
	Create(ctx context.Context, s *domain.Space) error
	Get(ctx context.Context, id string) (*domain.Space, error)
	GetByCode(ctx context.Context, workspaceID, code string) (*domain.Space, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.Space, error)
	Update(ctx context.Context, s *domain.Space) error
	SoftDelete(ctx context.Context, id string) error
}

type spaceRepo struct {
	db DBTX
}

// NewSpaceRepository wires a SpaceRepository to a DBTX (pool or tx).
func NewSpaceRepository(db *pgxpool.Pool) SpaceRepository { return &spaceRepo{db: db} }

func (r *spaceRepo) Create(ctx context.Context, s *domain.Space) error {
	if s == nil {
		return errors.New("repository: space is nil")
	}
	meta := s.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `INSERT INTO spaces (id, workspace_id, code, name, description, owner_id,
	                            embedding_model, embedding_dim, chunk_strategy,
	                            chunk_size, chunk_overlap, sag_enabled,
	                            metadata, created_at, updated_at, deleted_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5, $6,
	                   $7, $8, $9, $10, $11, $12, $13, NOW(), NOW(),
	                   '1970-01-01 00:00:00+00')`
	_, err := r.db.Exec(ctx, q,
		s.ID, s.WorkspaceID, s.Code, s.Name, nullable(s.Description), s.OwnerID,
		s.EmbeddingModel, s.EmbeddingDim, string(s.ChunkStrategy),
		s.ChunkSize, s.ChunkOverlap, s.SAGEnabled, meta,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("space code=%q in workspace=%q: %w", s.Code, s.WorkspaceID, mapped)
		}
		return fmt.Errorf("space.Create code=%q: %w", s.Code, err)
	}
	return nil
}

func (r *spaceRepo) Get(ctx context.Context, id string) (*domain.Space, error) {
	const q = `SELECT id, workspace_id, code, name, description, owner_id,
	                  embedding_model, embedding_dim, chunk_strategy,
	                  chunk_size, chunk_overlap, sag_enabled,
	                  metadata, created_at, updated_at, deleted_at
	           FROM spaces WHERE id = $1 AND deleted_at < '2100-01-01'`
	s, err := scanSpace(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("space.Get id=%s: %w", id, err)
	}
	return s, nil
}

func (r *spaceRepo) GetByCode(ctx context.Context, workspaceID, code string) (*domain.Space, error) {
	const q = `SELECT id, workspace_id, code, name, description, owner_id,
	                  embedding_model, embedding_dim, chunk_strategy,
	                  chunk_size, chunk_overlap, sag_enabled,
	                  metadata, created_at, updated_at, deleted_at
	           FROM spaces WHERE workspace_id = $1 AND code = $2 AND deleted_at < '2100-01-01'`
	s, err := scanSpace(r.db.QueryRow(ctx, q, workspaceID, code))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("space.GetByCode ws=%s code=%s: %w", workspaceID, code, err)
	}
	return s, nil
}

func (r *spaceRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.Space, error) {
	const q = `SELECT id, workspace_id, code, name, description, owner_id,
	                  embedding_model, embedding_dim, chunk_strategy,
	                  chunk_size, chunk_overlap, sag_enabled,
	                  metadata, created_at, updated_at, deleted_at
	           FROM spaces WHERE workspace_id = $1 AND deleted_at < '2100-01-01'
	           ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("space.ListByWorkspace ws=%s: %w", workspaceID, err)
	}
	defer rows.Close()
	var out []*domain.Space
	for rows.Next() {
		s, err := scanSpace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("space.ListByWorkspace rows.Err: %w", err)
	}
	return out, nil
}

func (r *spaceRepo) Update(ctx context.Context, s *domain.Space) error {
	if s == nil {
		return errors.New("repository: space is nil")
	}
	meta := s.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `UPDATE spaces
	           SET name = $2, description = $3, embedding_model = $4,
	               embedding_dim = $5, chunk_strategy = $6, chunk_size = $7,
	               chunk_overlap = $8, sag_enabled = $9, metadata = $10,
	               updated_at = NOW()
	           WHERE id = $1 AND deleted_at < '2100-01-01'`
	tag, err := r.db.Exec(ctx, q,
		s.ID, s.Name, nullable(s.Description), s.EmbeddingModel,
		s.EmbeddingDim, string(s.ChunkStrategy), s.ChunkSize,
		s.ChunkOverlap, s.SAGEnabled, meta,
	)
	if err != nil {
		return fmt.Errorf("space.Update id=%s: %w", s.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *spaceRepo) SoftDelete(ctx context.Context, id string) error {
	const q = `UPDATE spaces SET deleted_at = NOW(), updated_at = NOW()
	           WHERE id = $1 AND deleted_at < '2100-01-01'`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("space.SoftDelete id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSpace(row pgx.Row) (*domain.Space, error) {
	var s domain.Space
	var strategy string
	var deletedAt time.Time
	err := row.Scan(
		&s.ID, &s.WorkspaceID, &s.Code, &s.Name, &s.Description, &s.OwnerID,
		&s.EmbeddingModel, &s.EmbeddingDim, &strategy,
		&s.ChunkSize, &s.ChunkOverlap, &s.SAGEnabled,
		&s.Metadata, &s.CreatedAt, &s.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.ChunkStrategy = domain.ChunkStrategy(strategy)
	// Live rows carry the sentinel `1970-01-01 00:00:00+00`; surface
	// them as nil DeletedAt. Only real deletions (timestamp > 1970)
	// set the field.
	if !deletedAt.Equal(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t := deletedAt
		s.DeletedAt = &t
	}
	return &s, nil
}