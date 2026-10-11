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

// spaceRow matches the SELECT column order used by every space query.
// Column 15 is deleted_at (TIMESTAMPTZ). Live rows have the year 1970
// sentinel; the schema encodes the live marker as `deleted_at <
// '2100-01-01'`, which we mirror in scanSpace.
func spaceRow(id, wsID, code, name string, deletedAt time.Time) []any {
	createdAt := time.Now()
	updatedAt := createdAt
	return []any{
		id, wsID, code, name, "desc", "owner-1",
		"text-embedding-3", 1024, "semantic",
		512, 64, true,
		[]byte(`{}`), createdAt, updatedAt, deletedAt,
	}
}

func liveDeletedAt() time.Time { return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC) }
func trashedDeletedAt() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestSpaceRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &spaceRepo{db: m}
	s := &domain.Space{
		WorkspaceID: "ws-1", Code: "general", Name: "General",
		OwnerID: "u-1", EmbeddingModel: "text-embedding-3", EmbeddingDim: 1024,
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpaceRepository_Create_DuplicateCode(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &spaceRepo{db: m}
	err := repo.Create(context.Background(), &domain.Space{
		WorkspaceID: "ws-1", Code: "taken", Name: "x", OwnerID: "u",
		EmbeddingModel: "m", EmbeddingDim: 1024,
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestSpaceRepository_Create_NilSpace(t *testing.T) {
	m := &mockDBTX{}
	repo := &spaceRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil")
	}
}

func TestSpaceRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: spaceRow("s-1", "ws-1", "general", "General", liveDeletedAt())}}
	repo := &spaceRepo{db: m}
	got, err := repo.Get(context.Background(), "s-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "s-1" || got.EmbeddingDim != 1024 || got.IsDeleted() {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestSpaceRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &spaceRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSpaceRepository_Get_TrashedSurfacesDeletedAt(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: spaceRow("s-1", "ws-1", "x", "x", trashedDeletedAt())}}
	repo := &spaceRepo{db: m}
	got, err := repo.Get(context.Background(), "s-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsDeleted() {
		t.Fatalf("trashed row should set DeletedAt")
	}
}

func TestSpaceRepository_GetByCode_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: spaceRow("s-1", "ws-1", "general", "General", liveDeletedAt())}}
	repo := &spaceRepo{db: m}
	got, err := repo.GetByCode(context.Background(), "ws-1", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Code != "general" {
		t.Fatalf("bad code: %q", got.Code)
	}
}

func TestSpaceRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		spaceRow("s-1", "ws-1", "a", "A", liveDeletedAt()),
		spaceRow("s-2", "ws-1", "b", "B", liveDeletedAt()),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &spaceRepo{db: m}
	out, err := repo.ListByWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestSpaceRepository_List_QueryError(t *testing.T) {
	m := &mockDBTX{queryErr: errors.New("boom")}
	repo := &spaceRepo{db: m}
	if _, err := repo.ListByWorkspace(context.Background(), "ws"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSpaceRepository_Update_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &spaceRepo{db: m}
	s := &domain.Space{ID: "s-1", Name: "new", EmbeddingModel: "m", EmbeddingDim: 1024}
	if err := repo.Update(context.Background(), s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpaceRepository_Update_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &spaceRepo{db: m}
	if err := repo.Update(context.Background(), &domain.Space{ID: "missing", Name: "x", EmbeddingModel: "m", EmbeddingDim: 1024}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSpaceRepository_SoftDelete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &spaceRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "s-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpaceRepository_SoftDelete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &spaceRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}