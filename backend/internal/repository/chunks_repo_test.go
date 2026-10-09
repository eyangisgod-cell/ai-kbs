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

func chunkRow(id, docID string, ordinal int) []any {
	now := time.Now()
	return []any{
		id, docID, 1, ordinal, "content", "md5",
		nil, "text-embedding-3", 100, (*int)(nil),
		[]byte(`{}`), now,
	}
}

func TestChunkRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &chunkRepo{db: m}
	c := &domain.Chunk{DocumentID: "d-1", VersionNum: 1, Ordinal: 0, Content: "x", ContentMD5: "m", EmbeddingModel: "m", CharCount: 1}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChunkRepository_Create_DuplicateOrdinal(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &chunkRepo{db: m}
	c := &domain.Chunk{DocumentID: "d-1", VersionNum: 1, Ordinal: 0, Content: "x", ContentMD5: "m", EmbeddingModel: "m", CharCount: 1}
	if err := repo.Create(context.Background(), c); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestChunkRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &chunkRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestChunkRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: chunkRow("c-1", "d-1", 0)}}
	repo := &chunkRepo{db: m}
	c, err := repo.Get(context.Background(), "c-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Ordinal != 0 {
		t.Fatalf("bad row: %+v", c)
	}
}

func TestChunkRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &chunkRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestChunkRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		chunkRow("c-1", "d-1", 0),
		chunkRow("c-2", "d-1", 1),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &chunkRepo{db: m}
	out, err := repo.ListByDocumentVersion(context.Background(), "d-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestChunkRepository_Delete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 5")}
	repo := &chunkRepo{db: m}
	if err := repo.DeleteByDocumentVersion(context.Background(), "d-1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}