package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapPgError_Nil(t *testing.T) {
	if err := mapPgError(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMapPgError_NoRows(t *testing.T) {
	err := mapPgError(pgx.ErrNoRows)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMapPgError_UniqueViolation(t *testing.T) {
	in := &pgconn.PgError{Code: "23505", Message: "duplicate key value"}
	out := mapPgError(in)
	if !errors.Is(out, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", out)
	}
}

func TestMapPgError_ForeignKey(t *testing.T) {
	in := &pgconn.PgError{Code: "23503", Message: "FK violation"}
	out := mapPgError(in)
	if !errors.Is(out, ErrForeignKey) {
		t.Fatalf("expected ErrForeignKey, got %v", out)
	}
}

func TestMapPgError_OtherPgError(t *testing.T) {
	in := &pgconn.PgError{Code: "22001", Message: "string too long"}
	out := mapPgError(in)
	if errors.Is(out, ErrNotFound) || errors.Is(out, ErrAlreadyExists) || errors.Is(out, ErrForeignKey) {
		t.Fatalf("expected raw error to pass through, got %v", out)
	}
}

func TestMapPgError_GenericError(t *testing.T) {
	in := errors.New("some random failure")
	out := mapPgError(in)
	if out == nil || out.Error() != "some random failure" {
		t.Fatalf("expected passthrough, got %v", out)
	}
}

func TestVersionConflictError_Is(t *testing.T) {
	vce := &VersionConflictError{DocCode: "abc", BaseVersion: 1, CurrentVersion: 2}
	if !errors.Is(vce, ErrVersionConflict) {
		t.Fatalf("VersionConflictError must satisfy errors.Is(ErrVersionConflict)")
	}
	if vce.Error() == "" {
		t.Fatalf("expected non-empty message")
	}
}

func TestWrappedRepoError_Is(t *testing.T) {
	inner := errors.New("underlying")
	w := &wrappedRepoError{inner: inner, sentinel: ErrAlreadyExists}
	if !errors.Is(w, ErrAlreadyExists) {
		t.Fatalf("wrappedRepoError must satisfy errors.Is(sentinel)")
	}
	if w.Error() == "" {
		t.Fatalf("expected non-empty error")
	}
	if !errors.Is(w, inner) {
		t.Fatalf("wrappedRepoError must satisfy errors.Is(inner)")
	}
}

func TestWrapWith_NilInner(t *testing.T) {
	w := wrapWith(nil, ErrNotFound)
	if w.Error() != ErrNotFound.Error() {
		t.Fatalf("expected sentinel message")
	}
}

func TestWrapWith_ContextCancelled(t *testing.T) {
	// Sanity: context errors pass through unchanged.
	ctxErr := context.Canceled
	if err := mapPgError(ctxErr); err != ctxErr {
		t.Fatalf("context errors must not be remapped, got %v", err)
	}
}