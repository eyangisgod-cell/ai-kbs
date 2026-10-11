package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by every repository in this package.
//
// Callers MUST inspect them with `errors.Is`, never with string
// comparison — see `.trellis/spec/backend/error-handling.md` §"Typed
// Errors at Boundaries". Handlers translate these (wrapped or
// directly) into the envelope at the edge.
var (
	// ErrNotFound is returned when a Get / List-by-id query yields zero
	// rows. Maps to envelope code "doc.not_found" (HTTP 404).
	ErrNotFound = errors.New("repository: not found")

	// ErrAlreadyExists is returned when a Create / Insert collides with a
	// UNIQUE constraint (Postgres SQLSTATE 23505). Maps to envelope code
	// "internal.conflict" (HTTP 409).
	ErrAlreadyExists = errors.New("repository: already exists")

	// ErrVersionConflict is returned when an optimistic-CAS update fails
	// because the row's current version no longer matches the caller's
	// base_version. The full CAS semantics (base_version check + new
	// version bump) are owned by `internal/doc/cas.go` (T1.4); the
	// repository here returns the sentinel and the typed error so
	// downstream code can match without importing `internal/doc`.
	//
	// Maps to envelope code "doc.version_conflict" (HTTP 409).
	ErrVersionConflict = errors.New("repository: version conflict")

	// ErrForeignKey is returned when an INSERT / UPDATE violates a
	// foreign-key constraint (Postgres SQLSTATE 23503). Carries the
	// constraint name in the message; never the row contents.
	ErrForeignKey = errors.New("repository: foreign key violation")
)

// VersionConflictError is returned (in addition to the
// `ErrVersionConflict` sentinel) when a CAS update lost a race. The
// caller receives the BaseVersion it supplied and the CurrentVersion
// actually present in the row, so it can render a diff for the user.
//
// This is the typed shape the handler `errors.As`-checks to populate
// `meta.current_version` / `meta.base_version` per the envelope contract.
type VersionConflictError struct {
	DocCode        string
	BaseVersion    int
	CurrentVersion int
}

// Error implements the error interface.
func (e *VersionConflictError) Error() string {
	return "repository: version conflict"
}

// Is reports whether target is ErrVersionConflict so `errors.Is` works.
func (e *VersionConflictError) Is(target error) bool {
	return target == ErrVersionConflict
}

// DBTX is the small interface each repository depends on. It is satisfied
// by `*pgxpool.Pool` and by `pgx.Tx`, which lets repositories run inside a
// transaction without an extra abstraction layer.
//
// Keeping the surface tiny (only the methods every repo uses) is the
// whole point: it makes hand-written test mocks ~30 lines instead of
// thousands.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// mapPgError converts a pgx/pgconn error into the typed sentinel.
// Unknown errors are returned wrapped so the caller's forensic chain is
// preserved.
func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return wrapWith(err, ErrAlreadyExists)
		case "23503": // foreign_key_violation
			return wrapWith(err, ErrForeignKey)
		}
	}
	return err
}

// wrapWith attaches context to a sentinel while preserving Is/As matching.
func wrapWith(inner error, sentinel error) error {
	return &wrappedRepoError{inner: inner, sentinel: sentinel}
}

type wrappedRepoError struct {
	inner   error
	sentinel error
}

func (w *wrappedRepoError) Error() string {
	if w.inner == nil {
		return w.sentinel.Error()
	}
	return w.sentinel.Error() + ": " + w.inner.Error()
}

func (w *wrappedRepoError) Unwrap() error { return w.inner }

func (w *wrappedRepoError) Is(target error) bool {
	return target == w.sentinel || errors.Is(w.inner, target)
}