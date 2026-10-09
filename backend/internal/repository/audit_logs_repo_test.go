package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
)

func auditLogRow(id, wsID, action string) []any {
	now := time.Now()
	return []any{
		id, wsID, "u-1", "user", action,
		"document", "doc-1", "req-1", "trace-1",
		"127.0.0.1", "ua",
		[]byte(`{}`), now,
	}
}

func TestAuditLogRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &auditLogRepo{db: m}
	l := &domain.AuditLog{
		WorkspaceID: "ws-1",
		ActorID:     "u-1",
		ActorType:   domain.AuditActorUser,
		Action:      "document.create",
	}
	if err := repo.Create(context.Background(), l); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditLogRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &auditLogRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAuditLogRepository_ListByWorkspace_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		auditLogRow("l-1", "ws-1", "document.create"),
		auditLogRow("l-2", "ws-1", "document.update"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &auditLogRepo{db: m}
	out, err := repo.ListByWorkspace(context.Background(), "ws-1", 50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestAuditLogRepository_ListByWorkspace_DefaultsLimit(t *testing.T) {
	rows := mockRows{rows: [][]any{}}
	m := &mockDBTX{rows: &rows}
	repo := &auditLogRepo{db: m}
	if _, err := repo.ListByWorkspace(context.Background(), "ws-1", 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditLogRepository_ListByWorkspace_QueryErr(t *testing.T) {
	m := &mockDBTX{queryErr: errors.New("boom")}
	repo := &auditLogRepo{db: m}
	if _, err := repo.ListByWorkspace(context.Background(), "ws-1", 50, 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAuditLogRepository_ListByRequest_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		auditLogRow("l-1", "ws-1", "document.create"),
		auditLogRow("l-2", "ws-1", "document.update"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &auditLogRepo{db: m}
	out, err := repo.ListByRequest(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestAuditLogRepository_ListByRequest_Empty(t *testing.T) {
	// ListByRequest never returns ErrNotFound; an empty list is a
	// valid response. Verify that the happy path does not error.
	rows := mockRows{rows: [][]any{}}
	m := &mockDBTX{rows: &rows}
	repo := &auditLogRepo{db: m}
	out, err := repo.ListByRequest(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %d", len(out))
	}
}