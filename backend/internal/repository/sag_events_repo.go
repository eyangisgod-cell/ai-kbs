package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableSAGEvents is the SQL table backing SagEventRepository.
const TableSAGEvents = "sag_events"

// SagEventRepository persists SAG events.
type SagEventRepository interface {
	Create(ctx context.Context, e *domain.SAGEvent) error
	Get(ctx context.Context, id string) (*domain.SAGEvent, error)
	ListByDocument(ctx context.Context, documentID string) ([]*domain.SAGEvent, error)
	ListByChunk(ctx context.Context, chunkID string) ([]*domain.SAGEvent, error)
}

type sagEventRepo struct {
	db DBTX
}

// NewSagEventRepository wires a SagEventRepository to a DBTX (pool or tx).
func NewSagEventRepository(db *pgxpool.Pool) SagEventRepository {
	return &sagEventRepo{db: db}
}

func (r *sagEventRepo) Create(ctx context.Context, e *domain.SAGEvent) error {
	if e == nil {
		return errors.New("repository: sag event is nil")
	}
	meta := e.EventMetadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `INSERT INTO sag_events (id, chunk_id, document_id, event_text,
	                                  event_embedding, event_metadata,
	                                  occurred_at, created_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4,
	                   $5, $6,
	                   COALESCE($7, NOW()), NOW())`
	_, err := r.db.Exec(ctx, q,
		e.ID, e.ChunkID, e.DocumentID, e.EventText,
		e.EventEmbedding, meta, e.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("sag_event.Create doc=%s: %w", e.DocumentID, err)
	}
	return nil
}

func (r *sagEventRepo) Get(ctx context.Context, id string) (*domain.SAGEvent, error) {
	const q = `SELECT id, chunk_id, document_id, event_text, event_embedding,
	                  event_metadata, occurred_at, created_at
	           FROM sag_events WHERE id = $1`
	e, err := scanSAGEvent(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sag_event.Get id=%s: %w", id, err)
	}
	return e, nil
}

func (r *sagEventRepo) ListByDocument(ctx context.Context, documentID string) ([]*domain.SAGEvent, error) {
	const q = `SELECT id, chunk_id, document_id, event_text, event_embedding,
	                  event_metadata, occurred_at, created_at
	           FROM sag_events WHERE document_id = $1
	           ORDER BY occurred_at DESC`
	rows, err := r.db.Query(ctx, q, documentID)
	if err != nil {
		return nil, fmt.Errorf("sag_event.ListByDocument doc=%s: %w", documentID, err)
	}
	defer rows.Close()
	var out []*domain.SAGEvent
	for rows.Next() {
		e, err := scanSAGEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sag_event.ListByDocument rows.Err: %w", err)
	}
	return out, nil
}

func (r *sagEventRepo) ListByChunk(ctx context.Context, chunkID string) ([]*domain.SAGEvent, error) {
	const q = `SELECT id, chunk_id, document_id, event_text, event_embedding,
	                  event_metadata, occurred_at, created_at
	           FROM sag_events WHERE chunk_id = $1
	           ORDER BY occurred_at DESC`
	rows, err := r.db.Query(ctx, q, chunkID)
	if err != nil {
		return nil, fmt.Errorf("sag_event.ListByChunk chunk=%s: %w", chunkID, err)
	}
	defer rows.Close()
	var out []*domain.SAGEvent
	for rows.Next() {
		e, err := scanSAGEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sag_event.ListByChunk rows.Err: %w", err)
	}
	return out, nil
}

func scanSAGEvent(row pgx.Row) (*domain.SAGEvent, error) {
	var e domain.SAGEvent
	err := row.Scan(
		&e.ID, &e.ChunkID, &e.DocumentID, &e.EventText, &e.EventEmbedding,
		&e.EventMetadata, &e.OccurredAt, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}