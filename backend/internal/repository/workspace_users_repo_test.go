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

func workspaceUserRow(wsID, userID, role string) []any {
	now := time.Now()
	return []any{wsID, userID, role, (*string)(nil), now}
}

func TestWorkspaceUserRepository_Create_OK(t *testing.T) {
	m := &mockDBTX{}
	repo := &workspaceUserRepo{db: m}
	wu := &domain.WorkspaceUser{WorkspaceID: "ws-1", UserID: "u-1", Role: domain.WorkspaceRoleOwner}
	if err := repo.Create(context.Background(), wu); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceUserRepository_Create_Duplicate(t *testing.T) {
	m := &mockDBTX{execErr: &pgconn.PgError{Code: "23505"}}
	repo := &workspaceUserRepo{db: m}
	wu := &domain.WorkspaceUser{WorkspaceID: "ws-1", UserID: "u-1", Role: domain.WorkspaceRoleOwner}
	if err := repo.Create(context.Background(), wu); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestWorkspaceUserRepository_Create_Nil(t *testing.T) {
	m := &mockDBTX{}
	repo := &workspaceUserRepo{db: m}
	if err := repo.Create(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWorkspaceUserRepository_Get_OK(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanValues: workspaceUserRow("ws-1", "u-1", "owner")}}
	repo := &workspaceUserRepo{db: m}
	got, err := repo.Get(context.Background(), "ws-1", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Role != domain.WorkspaceRoleOwner {
		t.Fatalf("bad role: %v", got.Role)
	}
}

func TestWorkspaceUserRepository_Get_NotFound(t *testing.T) {
	m := &mockDBTX{row: &mockRow{scanErr: pgx.ErrNoRows}}
	repo := &workspaceUserRepo{db: m}
	if _, err := repo.Get(context.Background(), "ws-1", "u-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceUserRepository_ListByWorkspace_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{
		workspaceUserRow("ws-1", "u-1", "owner"),
		workspaceUserRow("ws-1", "u-2", "viewer"),
	}}
	m := &mockDBTX{rows: &rows}
	repo := &workspaceUserRepo{db: m}
	out, err := repo.ListByWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestWorkspaceUserRepository_ListByUser_OK(t *testing.T) {
	rows := mockRows{rows: [][]any{workspaceUserRow("ws-1", "u-1", "owner")}}
	m := &mockDBTX{rows: &rows}
	repo := &workspaceUserRepo{db: m}
	out, err := repo.ListByUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}

func TestWorkspaceUserRepository_UpdateRole_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 1")}
	repo := &workspaceUserRepo{db: m}
	if err := repo.UpdateRole(context.Background(), "ws-1", "u-1", domain.WorkspaceRoleAdmin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceUserRepository_UpdateRole_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("UPDATE 0")}
	repo := &workspaceUserRepo{db: m}
	if err := repo.UpdateRole(context.Background(), "ws-1", "u-1", domain.WorkspaceRoleAdmin); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceUserRepository_Delete_OK(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 1")}
	repo := &workspaceUserRepo{db: m}
	if err := repo.Delete(context.Background(), "ws-1", "u-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceUserRepository_Delete_NotFound(t *testing.T) {
	m := &mockDBTX{execTag: pgconn.NewCommandTag("DELETE 0")}
	repo := &workspaceUserRepo{db: m}
	if err := repo.Delete(context.Background(), "ws-1", "u-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}