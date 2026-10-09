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

func documentVersionRow(id, docID string, ver int) []any {
	now := time.Now()
	return []any{
		id, docID, ver, (*int)(nil), "md5", "content",
		(*string)(nil), int64(0), "author", "msg", (*int)(nil),
		[]byte(`{}`), now, false,
	}
}

func TestDocumentVersionRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &documentVersionRepo{db: m}
	v := &domain.DocumentVersion{DocumentID: "d-1", VersionNum: 1, MD5: "x", AuthorID: "u"}
	if err := repo.Create(context.Background(), v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentVersionRepository_Create_DuplicateVersion(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &documentVersionRepo{db: m}
	v := &domain.DocumentVersion{DocumentID: "d-1", VersionNum: 1, MD5: "x", AuthorID: "u"}
	if err := repo.Create(context.Background(), v); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestDocumentVersionRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &documentVersionRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDocumentVersionRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: documentVersionRow("v-1", "d-1", 1)}}
	repo := &documentVersionRepo{db: m}
	got, err := repo.Get(context.Background(), "v-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.VersionNum != 1 {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestDocumentVersionRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &documentVersionRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentVersionRepository_GetByVersion_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: documentVersionRow("v-2", "d-1", 2)}}
	repo := &documentVersionRepo{db: m}
	got, err := repo.GetByVersion(context.Background(), "d-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.VersionNum != 2 {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestDocumentVersionRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		documentVersionRow("v-2", "d-1", 2),
		documentVersionRow("v-1", "d-1", 1),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &documentVersionRepo{db: m}
	out, err := repo.ListByDocument(context.Background(), "d-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 || out[0].VersionNum != 2 {
		t.Fatalf("bad order: %+v", out)
	}
}

func TestDocumentVersionRepository_DeleteByDocumentVersion_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 1")}
	repo := &documentVersionRepo{db: m}
	if err := repo.DeleteByDocumentVersion(context.Background(), "d-1", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentVersionRepository_DeleteByDocumentVersion_NotFound(t *testing.T) {
	// 0 rows affected: must surface as ErrNotFound so the CAS
	// rollback path (T1.4) treats it as a benign no-op rather
	// than masking the original CAS failure.
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 0")}
	repo := &documentVersionRepo{db: m}
	if err := repo.DeleteByDocumentVersion(context.Background(), "missing", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}