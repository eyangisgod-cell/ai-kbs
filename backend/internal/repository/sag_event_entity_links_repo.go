package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableSAGEventEntityLinks is the SQL table backing SagEventEntityLinkRepository.
const TableSAGEventEntityLinks = "sag_event_entity_links"

// SagEventEntityLinkRepository persists the event ↔ entity link table.
type SagEventEntityLinkRepository interface {
	Create(ctx context.Context, l *domain.SAGEventEntityLink) error
	ListByEvent(ctx context.Context, eventID string) ([]*domain.SAGEventEntityLink, error)
	ListByEntity(ctx context.Context, entityID string) ([]*domain.SAGEventEntityLink, error)
	Delete(ctx context.Context, eventID, entityID string) error
}

type sagEventEntityLinkRepo struct {
	db DBTX
}

// NewSagEventEntityLinkRepository wires a SagEventEntityLinkRepository to
// a DBTX (pool or tx).
func NewSagEventEntityLinkRepository(db *pgxpool.Pool) SagEventEntityLinkRepository {
	return &sagEventEntityLinkRepo{db: db}
}

func (r *sagEventEntityLinkRepo) Create(ctx context.Context, l *domain.SAGEventEntityLink) error {
	if l == nil {
		return errors.New("repository: sag event entity link is nil")
	}
	const q = `INSERT INTO sag_event_entity_links (event_id, entity_id, weight, role)
	           VALUES ($1, $2, $3, $4)
	           ON CONFLICT (event_id, entity_id) DO UPDATE SET
	             weight = EXCLUDED.weight, role = EXCLUDED.role`
	_, err := r.db.Exec(ctx, q, l.EventID, l.EntityID, l.Weight, nullable(l.Role))
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrForeignKey) {
			return fmt.Errorf("sag_link event=%s entity=%s: %w", l.EventID, l.EntityID, mapped)
		}
		return fmt.Errorf("sag_link.Create event=%s: %w", l.EventID, err)
	}
	return nil
}

func (r *sagEventEntityLinkRepo) ListByEvent(ctx context.Context, eventID string) ([]*domain.SAGEventEntityLink, error) {
	const q = `SELECT event_id, entity_id, weight, role FROM sag_event_entity_links
	           WHERE event_id = $1 ORDER BY entity_id ASC`
	rows, err := r.db.Query(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("sag_link.ListByEvent event=%s: %w", eventID, err)
	}
	defer rows.Close()
	var out []*domain.SAGEventEntityLink
	for rows.Next() {
		l, err := scanSAGEventEntityLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sag_link.ListByEvent rows.Err: %w", err)
	}
	return out, nil
}

func (r *sagEventEntityLinkRepo) ListByEntity(ctx context.Context, entityID string) ([]*domain.SAGEventEntityLink, error) {
	const q = `SELECT event_id, entity_id, weight, role FROM sag_event_entity_links
	           WHERE entity_id = $1 ORDER BY event_id ASC`
	rows, err := r.db.Query(ctx, q, entityID)
	if err != nil {
		return nil, fmt.Errorf("sag_link.ListByEntity entity=%s: %w", entityID, err)
	}
	defer rows.Close()
	var out []*domain.SAGEventEntityLink
	for rows.Next() {
		l, err := scanSAGEventEntityLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sag_link.ListByEntity rows.Err: %w", err)
	}
	return out, nil
}

func (r *sagEventEntityLinkRepo) Delete(ctx context.Context, eventID, entityID string) error {
	const q = `DELETE FROM sag_event_entity_links WHERE event_id = $1 AND entity_id = $2`
	tag, err := r.db.Exec(ctx, q, eventID, entityID)
	if err != nil {
		return fmt.Errorf("sag_link.Delete event=%s entity=%s: %w", eventID, entityID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSAGEventEntityLink(row pgx.Row) (*domain.SAGEventEntityLink, error) {
	var l domain.SAGEventEntityLink
	var role *string
	err := row.Scan(&l.EventID, &l.EntityID, &l.Weight, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if role != nil {
		l.Role = *role
	}
	return &l, nil
}