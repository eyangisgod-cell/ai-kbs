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

// TableSessions is the SQL table backing SessionRepository.
const TableSessions = "sessions"

// SessionRepository persists chat/agent sessions.
type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, id string) (*domain.Session, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.Session, error)
	UpdateMetadata(ctx context.Context, id string, metadata []byte) error
	Delete(ctx context.Context, id string) error
}

type sessionRepo struct {
	db DBTX
}

// NewSessionRepository wires a SessionRepository to a DBTX (pool or tx).
func NewSessionRepository(db *pgxpool.Pool) SessionRepository { return &sessionRepo{db: db} }

func (r *sessionRepo) Create(ctx context.Context, s *domain.Session) error {
	if s == nil {
		return errors.New("repository: session is nil")
	}
	meta := s.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	kbIDs := s.KBIDs
	if kbIDs == nil {
		kbIDs = []string{}
	}
	const q = `INSERT INTO sessions (id, user_id, title, kb_ids, metadata,
	                                 created_at, updated_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   NOW(), NOW())`
	_, err := r.db.Exec(ctx, q, s.ID, s.UserID, nullable(s.Title), kbIDs, meta)
	if err != nil {
		return fmt.Errorf("session.Create user=%s: %w", s.UserID, err)
	}
	return nil
}

func (r *sessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	const q = `SELECT id, user_id, title, kb_ids, metadata, created_at, updated_at
	           FROM sessions WHERE id = $1`
	s, err := scanSession(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session.Get id=%s: %w", id, err)
	}
	return s, nil
}

func (r *sessionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	const q = `SELECT id, user_id, title, kb_ids, metadata, created_at, updated_at
	           FROM sessions WHERE user_id = $1
	           ORDER BY updated_at DESC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("session.ListByUser user=%s: %w", userID, err)
	}
	defer rows.Close()
	var out []*domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session.ListByUser rows.Err: %w", err)
	}
	return out, nil
}

func (r *sessionRepo) UpdateMetadata(ctx context.Context, id string, metadata []byte) error {
	meta := metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `UPDATE sessions SET metadata = $2, updated_at = NOW() WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, meta)
	if err != nil {
		return fmt.Errorf("session.UpdateMetadata id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sessionRepo) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM sessions WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("session.Delete id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSession(row pgx.Row) (*domain.Session, error) {
	var s domain.Session
	var title *string
	err := row.Scan(&s.ID, &s.UserID, &title, &s.KBIDs, &s.Metadata, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if title != nil {
		s.Title = *title
	}
	return &s, nil
}