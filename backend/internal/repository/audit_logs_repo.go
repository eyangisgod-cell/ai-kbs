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

// TableAuditLogs is the SQL table backing AuditLogRepository. Fork-compat:
// the WeKnora fork already owns `audit_logs`, so AI-KBS lives in
// `audit_logs_ai`. See package doc.go.
const TableAuditLogs = "audit_logs_ai"

// AuditLogRepository persists immutable audit rows. Persisted in the
// `audit_logs_ai` SQL table (fork-compat; see package doc).
//
// This repository NEVER logs payload / IP / User-Agent content; the
// auth middleware enforces redaction, but the repository reinforces
// that policy by emitting no log lines of its own.
type AuditLogRepository interface {
	Create(ctx context.Context, l *domain.AuditLog) error
	ListByWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*domain.AuditLog, error)
	ListByRequest(ctx context.Context, requestID string) ([]*domain.AuditLog, error)
}

type auditLogRepo struct {
	db DBTX
}

// NewAuditLogRepository wires an AuditLogRepository to a DBTX (pool or tx).
func NewAuditLogRepository(db *pgxpool.Pool) AuditLogRepository {
	return &auditLogRepo{db: db}
}

func (r *auditLogRepo) Create(ctx context.Context, l *domain.AuditLog) error {
	if l == nil {
		return errors.New("repository: audit log is nil")
	}
	payload := l.Payload
	if payload == nil {
		payload = json.RawMessage("{}")
	}
	const q = `INSERT INTO audit_logs_ai (id, workspace_id, actor_id, actor_type, action,
	                                     target_type, target_id, request_id, trace_id,
	                                     ip, user_agent, payload, created_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   $6, $7, $8, $9,
	                   $10, $11, $12, NOW())`
	_, err := r.db.Exec(ctx, q,
		l.ID, l.WorkspaceID, nullable(l.ActorID), string(l.ActorType), l.Action,
		nullable(l.TargetType), nullable(l.TargetID), nullable(l.RequestID), nullable(l.TraceID),
		nullable(l.IP), nullable(l.UserAgent), payload,
	)
	if err != nil {
		return fmt.Errorf("audit_log.Create ws=%s: %w", l.WorkspaceID, err)
	}
	return nil
}

func (r *auditLogRepo) ListByWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*domain.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	const q = `SELECT id, workspace_id, actor_id, actor_type, action,
	                  target_type, target_id, request_id, trace_id,
	                  ip, user_agent, payload, created_at
	           FROM audit_logs_ai WHERE workspace_id = $1
	           ORDER BY created_at DESC
	           LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, workspaceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit_log.ListByWorkspace ws=%s: %w", workspaceID, err)
	}
	defer rows.Close()
	var out []*domain.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit_log.ListByWorkspace rows.Err: %w", err)
	}
	return out, nil
}

func (r *auditLogRepo) ListByRequest(ctx context.Context, requestID string) ([]*domain.AuditLog, error) {
	const q = `SELECT id, workspace_id, actor_id, actor_type, action,
	                  target_type, target_id, request_id, trace_id,
	                  ip, user_agent, payload, created_at
	           FROM audit_logs_ai WHERE request_id = $1
	           ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, requestID)
	if err != nil {
		return nil, fmt.Errorf("audit_log.ListByRequest %q: %w", requestID, err)
	}
	defer rows.Close()
	var out []*domain.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit_log.ListByRequest rows.Err: %w", err)
	}
	return out, nil
}

func scanAuditLog(row pgx.Row) (*domain.AuditLog, error) {
	var l domain.AuditLog
	var actorID *string
	var actorType string
	var targetType, targetID, requestID, traceID *string
	var ip, userAgent *string
	err := row.Scan(
		&l.ID, &l.WorkspaceID, &actorID, &actorType, &l.Action,
		&targetType, &targetID, &requestID, &traceID,
		&ip, &userAgent, &l.Payload, &l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if actorID != nil {
		l.ActorID = *actorID
	}
	l.ActorType = domain.AuditActorType(actorType)
	if targetType != nil {
		l.TargetType = *targetType
	}
	if targetID != nil {
		l.TargetID = *targetID
	}
	if requestID != nil {
		l.RequestID = *requestID
	}
	if traceID != nil {
		l.TraceID = *traceID
	}
	if ip != nil {
		l.IP = *ip
	}
	if userAgent != nil {
		l.UserAgent = *userAgent
	}
	return &l, nil
}