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

// workspaceRow is the canonical column order returned by every
// workspace SELECT. Keep aligned with scanWorkspace.
func workspaceRow(id, name, slug, ownerID string, deletedAt *time.Time) []any {
	createdAt := time.Now()
	updatedAt := createdAt
	if deletedAt != nil {
		return []any{id, name, slug, ownerID, createdAt, updatedAt, *deletedAt}
	}
	return []any{id, name, slug, ownerID, createdAt, updatedAt, nil}
}

func TestWorkspaceRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &workspaceRepo{db: m}
	w := &domain.Workspace{Name: "My Workspace", Slug: "my-ws", OwnerID: "user-1"}
	if err := repo.Create(context.Background(), w); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.execCount != 1 {
		t.Fatalf("exec count = %d, want 1", m.execCount)
	}
}

func TestWorkspaceRepository_Create_DuplicateSlug(t *testing.T) {
	m := &mockDBTX{
		execErr: &pgconn.PgError{Code: "23505", Message: "duplicate key value"},
	}
	repo := &workspaceRepo{db: m}
	w := &domain.Workspace{Name: "x", Slug: "taken", OwnerID: "u"}
	err := repo.Create(context.Background(), w)
	if err == nil || !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestWorkspaceRepository_Create_UnexpectedError(t *testing.T) {
	other := errors.New("connection refused")
	m := &mockDBTX{execErr: other}
	repo := &workspaceRepo{db: m}
	err := repo.Create(context.Background(), &domain.Workspace{Name: "x", Slug: "y", OwnerID: "u"})
	if err == nil || errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected raw wrapped error, got %v", err)
	}
}

func TestWorkspaceRepository_Create_NilWorkspace(t *testing.T) {
	m := &mockDBTX{}
	repo := &workspaceRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil workspace")
	}
}

func TestWorkspaceRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{
		row: &mockRow{scanValues: workspaceRow("w1", "My", "my", "u1", nil)},
	}
	repo := &workspaceRepo{db: m}
	got, err := repo.Get(context.Background(), "w1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "w1" || got.Slug != "my" || got.IsDeleted() {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestWorkspaceRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &workspaceRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceRepository_GetBySlug_OK(t *testing.T) {
	m := &mockDBTX{
		row: &mockRow{scanValues: workspaceRow("w1", "My", "my", "u1", nil)},
	}
	repo := &workspaceRepo{db: m}
	got, err := repo.GetBySlug(context.Background(), "my")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "my" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestWorkspaceRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		workspaceRow("w1", "My", "my", "u1", nil),
		workspaceRow("w2", "Other", "other", "u1", nil),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &workspaceRepo{db: m}
	out, err := repo.List(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].ID != "w1" || out[1].ID != "w2" {
		t.Fatalf("ordering wrong: %+v", out)
	}
}

func TestWorkspaceRepository_List_QueryError(t *testing.T) {
	m := &mockDBTX{queryErr: errors.New("boom")}
	repo := &workspaceRepo{db: m}
	if _, err := repo.List(context.Background(), "u1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWorkspaceRepository_List_RowsError(t *testing.T) {
	rows := mockRows{rows: [][]any{workspaceRow("w1", "x", "x", "u", nil)}, errAfter: errors.New("rows fail")}
	m := &mockDBTX{rows: &rows}
	repo := &workspaceRepo{db: m}
	if _, err := repo.List(context.Background(), "u"); err == nil {
		t.Fatalf("expected rows.Err to surface")
	}
}

func TestWorkspaceRepository_Update_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &workspaceRepo{db: m}
	err := repo.Update(context.Background(), &domain.Workspace{ID: "w1", Name: "new", Slug: "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceRepository_Update_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &workspaceRepo{db: m}
	if err := repo.Update(context.Background(), &domain.Workspace{ID: "missing", Name: "x", Slug: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceRepository_SoftDelete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &workspaceRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "w1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceRepository_SoftDelete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &workspaceRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}