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

func userRow(id, username string) []any {
	now := time.Now()
	return []any{
		id, username, "u@example.com", "Display",
		"$2a$10$abcdef", false,
		now, now, false,
	}
}

func TestUserRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &userRepo{db: m}
	u := &domain.User{Username: "alice", Email: "a@x", DisplayName: "Alice", PasswordHash: "h"}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserRepository_Create_DuplicateUsername(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &userRepo{db: m}
	err := repo.Create(context.Background(), &domain.User{Username: "alice"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUserRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &userRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUserRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: userRow("u-1", "alice")}}
	repo := &userRepo{db: m}
	got, err := repo.Get(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != "alice" || got.Email != "u@example.com" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestUserRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &userRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_GetByUsername_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: userRow("u-1", "alice")}}
	repo := &userRepo{db: m}
	got, err := repo.GetByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != "alice" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestUserRepository_GetByEmail_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: userRow("u-1", "alice")}}
	repo := &userRepo{db: m}
	got, err := repo.GetByEmail(context.Background(), "u@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != "alice" {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestUserRepository_UpdatePasswordHash_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &userRepo{db: m}
	if err := repo.UpdatePasswordHash(context.Background(), "u-1", "new-hash"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserRepository_UpdatePasswordHash_Empty(t *testing.T) {
	m := &mockDBTX{}
	repo := &userRepo{db: m}
	if err := repo.UpdatePasswordHash(context.Background(), "u-1", ""); err == nil {
		t.Fatalf("expected error for empty hash")
	}
}

func TestUserRepository_UpdatePasswordHash_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &userRepo{db: m}
	if err := repo.UpdatePasswordHash(context.Background(), "missing", "h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_SoftDelete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &userRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "u-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserRepository_SoftDelete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &userRepo{db: m}
	if err := repo.SoftDelete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}