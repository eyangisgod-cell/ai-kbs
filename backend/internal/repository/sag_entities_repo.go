package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableSAGEntities is the SQL table backing SagEntityRepository.
const TableSAGEntities = "sag_entities"

// SagEntityRepository persists SAG entities.
type SagEntityRepository interface {
	Create(ctx context.Context, e *domain.SAGEntity) error
	Get(ctx context.Context, id string) (*domain.SAGEntity, error)
	GetByName(ctx context.Context, spaceID, name string) (*domain.SAGEntity, error)
	ListBySpace(ctx context.Context, spaceID string) ([]*domain.SAGEntity, error)
	UpdateAliases(ctx context.Context, id string, aliases []string) error
}

type sagEntityRepo struct {
	db DBTX
}

// NewSagEntityRepository wires a SagEntityRepository to a DBTX (pool or tx).
func NewSagEntityRepository(db *pgxpool.Pool) SagEntityRepository {
	return &sagEntityRepo{db: db}
}

func (r *sagEntityRepo) Create(ctx context.Context, e *domain.SAGEntity) error {
	if e == nil {
		return errors.New("repository: sag entity is nil")
	}
	aliases := e.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	const q = `INSERT INTO sag_entities (id, space_id, name, entity_type, description,
	                                     canonical_id, aliases, embedding, created_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   $6, $7, $8, NOW())`
	_, err := r.db.Exec(ctx, q,
		e.ID, e.SpaceID, e.Name, nullable(e.EntityType), nullable(e.Description),
		e.CanonicalID, aliases, e.Embedding,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("sag_entity name=%q in space=%q: %w", e.Name, e.SpaceID, mapped)
		}
		return fmt.Errorf("sag_entity.Create name=%q: %w", e.Name, err)
	}
	return nil
}

func (r *sagEntityRepo) Get(ctx context.Context, id string) (*domain.SAGEntity, error) {
	const q = `SELECT id, space_id, name, entity_type, description, canonical_id,
	                  aliases, embedding, created_at
	           FROM sag_entities WHERE id = $1`
	e, err := scanSAGEntity(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sag_entity.Get id=%s: %w", id, err)
	}
	return e, nil
}

func (r *sagEntityRepo) GetByName(ctx context.Context, spaceID, name string) (*domain.SAGEntity, error) {
	const q = `SELECT id, space_id, name, entity_type, description, canonical_id,
	                  aliases, embedding, created_at
	           FROM sag_entities WHERE space_id = $1 AND name = $2`
	e, err := scanSAGEntity(r.db.QueryRow(ctx, q, spaceID, name))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sag_entity.GetByName space=%s name=%s: %w", spaceID, name, err)
	}
	return e, nil
}

func (r *sagEntityRepo) ListBySpace(ctx context.Context, spaceID string) ([]*domain.SAGEntity, error) {
	const q = `SELECT id, space_id, name, entity_type, description, canonical_id,
	                  aliases, embedding, created_at
	           FROM sag_entities WHERE space_id = $1
	           ORDER BY name ASC`
	rows, err := r.db.Query(ctx, q, spaceID)
	if err != nil {
		return nil, fmt.Errorf("sag_entity.ListBySpace space=%s: %w", spaceID, err)
	}
	defer rows.Close()
	var out []*domain.SAGEntity
	for rows.Next() {
		e, err := scanSAGEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sag_entity.ListBySpace rows.Err: %w", err)
	}
	return out, nil
}

func (r *sagEntityRepo) UpdateAliases(ctx context.Context, id string, aliases []string) error {
	if aliases == nil {
		aliases = []string{}
	}
	const q = `UPDATE sag_entities SET aliases = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, aliases)
	if err != nil {
		return fmt.Errorf("sag_entity.UpdateAliases id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSAGEntity(row pgx.Row) (*domain.SAGEntity, error) {
	var e domain.SAGEntity
	var entityType, description *string
	err := row.Scan(
		&e.ID, &e.SpaceID, &e.Name, &entityType, &description,
		&e.CanonicalID, &e.Aliases, &e.Embedding, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if entityType != nil {
		e.EntityType = *entityType
	}
	if description != nil {
		e.Description = *description
	}
	return &e, nil
}