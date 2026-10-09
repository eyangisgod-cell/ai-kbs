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

// documentRow mirrors the documents SELECT column order.
func documentRow(id, spaceID, docCode, title string, activeVer int, deletedFlag bool) []any {
	now := time.Now()
	var deletedAt any = liveDeletedAt()
	if deletedFlag {
		deletedAt = now
	}
	return []any{
		id, spaceID, docCode, (*string)(nil), "/", 0,
		title, "markdown", "text/markdown", activeVer, "md5",
		(*string)(nil), int64(0),
		false, false,
		[]byte(nil), []string{"t1", "t2"}, []byte(`{"k":"v"}`), "unclassified",
		"creator", "updater", now, now,
		deletedFlag, deletedAt,
	}
}

func TestDocumentRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &documentRepo{db: m}
	d := &domain.Document{
		SpaceID: "sp-1", DocCode: "abc", Title: "Hello",
		ActiveVersion: 1, ActiveMD5: "x",
		CreatedBy: "u1", UpdatedBy: "u1",
	}
	if err := repo.Create(context.Background(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentRepository_Create_MissingDocCode(t *testing.T) {
	m := &mockDBTX{}
	repo := &documentRepo{db: m}
	err := repo.Create(context.Background(), &domain.Document{SpaceID: "sp", Title: "x", CreatedBy: "u", UpdatedBy: "u"})
	if err == nil {
		t.Fatalf("expected error for missing DocCode")
	}
}

func TestDocumentRepository_Create_DuplicateCode(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &documentRepo{db: m}
	err := repo.Create(context.Background(), &domain.Document{
		SpaceID: "sp", DocCode: "x", Title: "x", CreatedBy: "u", UpdatedBy: "u",
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestDocumentRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: documentRow("d-1", "sp-1", "abc", "Hello", 1, false)}}
	repo := &documentRepo{db: m}
	d, err := repo.Get(context.Background(), "d-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.ID != "d-1" || d.DocCode != "abc" || len(d.Tags) != 2 {
		t.Fatalf("bad row: %+v", d)
	}
}

func TestDocumentRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &documentRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentRepository_GetByCode_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: documentRow("d-1", "sp-1", "abc", "Hi", 2, false)}}
	repo := &documentRepo{db: m}
	d, err := repo.GetByCode(context.Background(), "sp-1", "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.ActiveVersion != 2 {
		t.Fatalf("bad active_version: %d", d.ActiveVersion)
	}
}

func TestDocumentRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		documentRow("d-1", "sp-1", "a", "A", 1, false),
		documentRow("d-2", "sp-1", "b", "B", 1, false),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &documentRepo{db: m}
	out, err := repo.ListBySpace(context.Background(), "sp-1", 50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestDocumentRepository_List_DefaultsLimit(t *testing.T) {
	rows := mockRows{rows: [][]any{}}
	m := &mockDBTX{rows: &rows}
	repo := &documentRepo{db: m}
	if _, err := repo.ListBySpace(context.Background(), "sp-1", 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentRepository_Update_OK(t *testing.T) {
	m := &mockDBTX{
		row: &mockRow{scanValues: []any{7}},
	}
	repo := &documentRepo{db: m}
	d := &domain.Document{ID: "d-1", DocCode: "abc", Title: "new", ActiveMD5: "y", UpdatedBy: "u"}
	newVer, err := repo.Update(context.Background(), d, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newVer != 7 {
		t.Fatalf("newVer=%d want 7", newVer)
	}
}

func TestDocumentRepository_Update_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &documentRepo{db: m}
	_, err := repo.Update(context.Background(), &domain.Document{ID: "missing", Title: "x", ActiveMD5: "y", UpdatedBy: "u"}, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentRepository_Update_VersionConflict(t *testing.T) {
	m := &multiRowDBTX{rows: []pgx.Row{
		&mockRow{scanErr: pgx.ErrNoRows},      // UPDATE miss
		&mockRow{scanValues: []any{int64(8)}}, // classify read
	}}
	repo := &documentRepo{db: m}
	d := &domain.Document{ID: "d-1", Title: "new", ActiveMD5: "y", UpdatedBy: "u"}
	_, err := repo.Update(context.Background(), d, 5)
	vce, ok := err.(*VersionConflictError)
	if !ok {
		t.Fatalf("expected *VersionConflictError, got %T %v", err, err)
	}
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	if vce.CurrentVersion != 8 || vce.BaseVersion != 5 {
		t.Fatalf("bad vce: %+v", vce)
	}
}

func TestDocumentRepository_Update_NilDocument(t *testing.T) {
	m := &mockDBTX{}
	repo := &documentRepo{db: m}
	if _, err := repo.Update(context.Background(), nil, 1); err == nil {
		t.Fatalf("expected error for nil document")
	}
}

func TestDocumentRepository_SoftDelete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &documentRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "d-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentRepository_SoftDelete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &documentRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentRepository_GetCurrentVersion_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &documentRepo{db: m}
	if _, err := repo.GetCurrentVersion(context.Background(), "d-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// multiRowDBTX returns different pgx.Row instances on successive
// QueryRow calls. Used by TestDocumentRepository_Update_VersionConflict.
type multiRowDBTX struct {
	rows []pgx.Row
	idx  int
}

func (m *multiRowDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *multiRowDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}
func (m *multiRowDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r := m.rows[m.idx]
	m.idx++
	return r
}