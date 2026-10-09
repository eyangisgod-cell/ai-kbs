package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
)

func sagEventRow(id, chunkID, docID string) []any {
	now := time.Now()
	return []any{
		id, chunkID, docID, "event text",
		nil, []byte(`{}`),
		now, now,
	}
}

func TestSagEventRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEventRepo{db: m}
	e := &domain.SAGEvent{ChunkID: "c-1", DocumentID: "d-1", EventText: "x"}
	if err := repo.Create(context.Background(), e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSagEventRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEventRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSagEventRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: sagEventRow("e-1", "c-1", "d-1")}}
	repo := &sagEventRepo{db: m}
	got, err := repo.Get(context.Background(), "e-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EventText != "event text" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestSagEventRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &sagEventRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSagEventRepository_ListByDocument_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		sagEventRow("e-1", "c-1", "d-1"),
		sagEventRow("e-2", "c-2", "d-1"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &sagEventRepo{db: m}
	out, err := repo.ListByDocument(context.Background(), "d-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestSagEventRepository_ListByChunk_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{sagEventRow("e-1", "c-1", "d-1")}}
	m := &mockDBTX{rows: &rows}
	repo := &sagEventRepo{db: m}
	out, err := repo.ListByChunk(context.Background(), "c-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestSagEventRepository_ListByChunk_QueryErr(t *testing.T) {
	m := &mockDBTX{queryErr: errors.New("boom")}
	repo := &sagEventRepo{db: m}
	if _, err := repo.ListByChunk(context.Background(), "c-1"); err == nil {
		t.Fatalf("expected error")
	}
}