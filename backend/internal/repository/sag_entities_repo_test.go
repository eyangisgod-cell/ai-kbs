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

func sagEntityRow(id, spaceID, name string) []any {
	now := time.Now()
	return []any{
		id, spaceID, name, "person", "desc",
		(*string)(nil), []string{"a1", "a2"}, nil, now,
	}
}

func TestSagEntityRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEntityRepo{db: m}
	e := &domain.SAGEntity{SpaceID: "sp-1", Name: "Alice", EntityType: "person"}
	if err := repo.Create(context.Background(), e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSagEntityRepository_Create_DuplicateName(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &sagEntityRepo{db: m}
	e := &domain.SAGEntity{SpaceID: "sp-1", Name: "Alice"}
	if err := repo.Create(context.Background(), e); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestSagEntityRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &sagEntityRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSagEntityRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: sagEntityRow("e-1", "sp-1", "Alice")}}
	repo := &sagEntityRepo{db: m}
	got, err := repo.Get(context.Background(), "e-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Alice" || got.EntityType != "person" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestSagEntityRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &sagEntityRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSagEntityRepository_GetByName_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: sagEntityRow("e-1", "sp-1", "Alice")}}
	repo := &sagEntityRepo{db: m}
	got, err := repo.GetByName(context.Background(), "sp-1", "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Alice" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestSagEntityRepository_List_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		sagEntityRow("e-1", "sp-1", "Alice"),
		sagEntityRow("e-2", "sp-1", "Bob"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &sagEntityRepo{db: m}
	out, err := repo.ListBySpace(context.Background(), "sp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestSagEntityRepository_UpdateAliases_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &sagEntityRepo{db: m}
	if err := repo.UpdateAliases(context.Background(), "e-1", []string{"x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSagEntityRepository_UpdateAliases_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &sagEntityRepo{db: m}
	if err := repo.UpdateAliases(context.Background(), "missing", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}