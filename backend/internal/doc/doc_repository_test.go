// External tests for the RepositoryLookup adapter. Lives in the
// `doc_test` package so it can import `internal/repository` without
// creating an import cycle (the production `doc` package is already
// dependent on it; the test file isn't, but imports are resolved at
// compile time across the package anyway). External package is the
// idiomatic Go workaround.
package doc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eyangisgod-cell/ai-kbs/internal/doc"
	"github.com/eyangisgod-cell/ai-kbs/internal/repository"
)

func TestRepositoryLookup_NotFound(t *testing.T) {
	t.Parallel()
	// Verifies that RepositoryLookup translates
	// repository.ErrNotFound into (nil, nil). The adapter accepts a
	// function so we don't need to fake the entire
	// DocumentRepository interface (7 methods).
	adapted := doc.RepositoryLookup(func(ctx context.Context, spaceID, code string) (any, error) {
		return nil, repository.ErrNotFound
	})
	existing, err := adapted.LookupByCode(context.Background(), "sp-1", "abc")
	if err != nil {
		t.Fatalf("expected nil error on ErrNotFound, got %v", err)
	}
	if existing != nil {
		t.Fatalf("expected nil existing, got %v", existing)
	}
}

func TestRepositoryLookup_PropagatesOtherError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	adapted := doc.RepositoryLookup(func(ctx context.Context, spaceID, code string) (any, error) {
		return nil, boom
	})
	_, err := adapted.LookupByCode(context.Background(), "sp-1", "abc")
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestRepositoryLookup_PropagatesDocument(t *testing.T) {
	t.Parallel()
	adapted := doc.RepositoryLookup(func(ctx context.Context, spaceID, code string) (any, error) {
		return struct{ ID, SpaceID, DocCode string }{ID: "d-1", SpaceID: "sp-1", DocCode: code}, nil
	})
	existing, err := adapted.LookupByCode(context.Background(), "sp-1", "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existing == nil {
		t.Fatalf("expected non-nil existing")
	}
}

func TestRepositoryLookup_NilDocument(t *testing.T) {
	t.Parallel()
	adapted := doc.RepositoryLookup(func(ctx context.Context, spaceID, code string) (any, error) {
		return nil, nil
	})
	existing, err := adapted.LookupByCode(context.Background(), "sp-1", "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existing != nil {
		t.Fatalf("expected nil existing, got %v", existing)
	}
}
