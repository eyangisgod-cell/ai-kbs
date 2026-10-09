package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func sagLinkRow(eventID, entityID string) []any {
	return []any{eventID, entityID, float32(0.5), "subject"}
}

func TestSagLinkRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEventEntityLinkRepo{db: m}
	l := &domain.SAGEventEntityLink{EventID: "e-1", EntityID: "en-1", Weight: 0.5, Role: "subject"}
	if err := repo.Create(context.Background(), l); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSagLinkRepository_Create_ForeignKey(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23503"}}
	repo := &sagEventEntityLinkRepo{db: m}
	l := &domain.SAGEventEntityLink{EventID: "missing", EntityID: "missing"}
	if err := repo.Create(context.Background(), l); !errors.Is(err, ErrForeignKey) {
		t.Fatalf("expected ErrForeignKey, got %v", err)
	}
}

func TestSagLinkRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEventEntityLinkRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSagLinkRepository_ListByEvent_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		sagLinkRow("e-1", "en-1"),
		sagLinkRow("e-1", "en-2"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &sagEventEntityLinkRepo{db: m}
	out, err := repo.ListByEvent(context.Background(), "e-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestSagLinkRepository_ListByEntity_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{sagLinkRow("e-1", "en-1")}}
	m := &mockDBTX{rows: &rows}
	repo := &sagEventEntityLinkRepo{db: m}
	out, err := repo.ListByEntity(context.Background(), "en-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestSagLinkRepository_Delete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 1")}
	repo := &sagEventEntityLinkRepo{db: m}
	if err := repo.Delete(context.Background(), "e-1", "en-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSagLinkRepository_Delete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 0")}
	repo := &sagEventEntityLinkRepo{db: m}
	if err := repo.Delete(context.Background(), "e-1", "en-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSagLinkRepository_ListByEvent_QueryErr(t *testing.T) {
	m := &mockDBTX{queryErr: errors.New("boom")}
	repo := &sagEventEntityLinkRepo{db: m}
	if _, err := repo.ListByEvent(context.Background(), "e-1"); err == nil {
		t.Fatalf("expected error")
	}
}