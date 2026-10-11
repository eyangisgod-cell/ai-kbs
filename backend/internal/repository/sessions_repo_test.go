package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func sessionRow(id, userID, title string) []any {
	now := time.Now()
	return []any{
		id, userID, (*string)(&title), []string{"sp-1"}, []byte(`{}`), now, now,
	}
}

func TestSessionRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &sessionRepo{db: m}
	s := &domain.Session{UserID: "u-1", Title: "t", KBIDs: []string{"sp-1"}}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &sessionRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSessionRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: sessionRow("s-1", "u-1", "title")}}
	repo := &sessionRepo{db: m}
	got, err := repo.Get(context.Background(), "s-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "u-1" || got.Title != "title" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestSessionRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &sessionRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionRepository_ListByUser_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{sessionRow("s-1", "u-1", "x")}}
	m := &mockDBTX{rows: &rows}
	repo := &sessionRepo{db: m}
	out, err := repo.ListByUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestSessionRepository_UpdateMetadata_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &sessionRepo{db: m}
	if err := repo.UpdateMetadata(context.Background(), "s-1", []byte(`{"x":1}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRepository_UpdateMetadata_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &sessionRepo{db: m}
	if err := repo.UpdateMetadata(context.Background(), "missing", []byte(`{}`)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionRepository_Delete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 1")}
	repo := &sessionRepo{db: m}
	if err := repo.Delete(context.Background(), "s-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRepository_Delete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 0")}
	repo := &sessionRepo{db: m}
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}