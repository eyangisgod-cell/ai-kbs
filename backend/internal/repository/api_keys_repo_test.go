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

func apiKeyRow(id, keyID, name string) []any {
	now := time.Now()
	return []any{
		id, "u-1", "ws-1", keyID, "$2a$10$hash", name,
		[]string{"read"}, []string{"sp-1"}, (*time.Time)(nil), (*time.Time)(nil),
		now, (*time.Time)(nil),
	}
}

func TestAPIKeyRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &apiKeyRepo{db: m}
	k := &domain.APIKey{UserID: "u-1", WorkspaceID: "ws-1", KeyID: "k-1", KeyHash: "h", Name: "my-key"}
	if err := repo.Create(context.Background(), k); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIKeyRepository_Create_MissingFields(t *testing.T) {
	m := &mockDBTX{}
	repo := &apiKeyRepo{db: m}
	if err := repo.Create(context.Background(), &domain.APIKey{UserID: "u", WorkspaceID: "ws"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIKeyRepository_Create_DuplicateKeyID(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &apiKeyRepo{db: m}
	k := &domain.APIKey{UserID: "u-1", WorkspaceID: "ws-1", KeyID: "k-1", KeyHash: "h", Name: "x"}
	if err := repo.Create(context.Background(), k); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestAPIKeyRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &apiKeyRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIKeyRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: apiKeyRow("k-1", "kid", "n")}}
	repo := &apiKeyRepo{db: m}
	got, err := repo.Get(context.Background(), "k-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.KeyID != "kid" || len(got.Capabilities) != 1 {
		t.Fatalf("bad row: %+v", got)
	}
}

func TestAPIKeyRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &apiKeyRepo{db: m}
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAPIKeyRepository_GetByKeyID_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: apiKeyRow("k-1", "kid", "n")}}
	repo := &apiKeyRepo{db: m}
	got, err := repo.GetByKeyID(context.Background(), "kid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.KeyHash == "" {
		t.Fatalf("expected key hash to be returned")
	}
}

func TestAPIKeyRepository_ListByUser_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{apiKeyRow("k-1", "kid", "n")}}
	m := &mockDBTX{rows: &rows}
	repo := &apiKeyRepo{db: m}
	out, err := repo.ListByUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestAPIKeyRepository_ListByWorkspace_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{apiKeyRow("k-1", "kid", "n")}}
	m := &mockDBTX{rows: &rows}
	repo := &apiKeyRepo{db: m}
	out, err := repo.ListByWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestAPIKeyRepository_Revoke_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &apiKeyRepo{db: m}
	if err := repo.Revoke(context.Background(), "k-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIKeyRepository_Revoke_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &apiKeyRepo{db: m}
	if err := repo.Revoke(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAPIKeyRepository_TouchLastUsed_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &apiKeyRepo{db: m}
	if err := repo.TouchLastUsed(context.Background(), "k-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}