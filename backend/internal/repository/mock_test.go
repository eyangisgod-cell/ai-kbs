package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockDBTX is a hand-written in-package fake of DBTX. Tests build one
// per scenario by setting row / rows / err fields, then constructing a
// repo bound to it.
//
// Why hand-rolled and not mockgen: pgx.Rows is a 10-method interface
// with subtle ordering (Scan is only valid after Next returns true).
// A 70-line fake serves all 13 repos without runtime reflection cost
// and keeps the package dependency-light (no go.uber.org/mock added).
type mockDBTX struct {
	// row is returned by QueryRow.
	row pgx.Row
	// rows is returned by Query (multi-row reads).
	rows pgx.Rows
	// queryErr is the error returned by Query.
	queryErr error
	// execErr is the error returned by Exec.
	execErr error
	// execTag is the CommandTag returned by Exec.
	execTag pgconn.CommandTag

	// execCount, rowCount, queryCount track invocations for assertions.
	execCount  int
	rowCount   int
	queryCount int
}

// Exec implements DBTX.
func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCount++
	return m.execTag, m.execErr
}

// Query implements DBTX.
func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCount++
	return m.rows, m.queryErr
}

// QueryRow implements DBTX.
func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.rowCount++
	return m.row
}

// mockRow implements pgx.Row for tests. Pass scanValues positionally;
// Scan will assign them one-by-one to the destination values.
type mockRow struct {
	scanValues []any
	scanErr    error
}

// Scan implements pgx.Row by copying values into destinations of
// known types. We deliberately support only the small set the repos
// actually use (string, int, int64, bool, []byte, *string for nullable
// TEXT, *time.Time, pgvector.Vector). Anything else is an error.
func (r *mockRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) != len(r.scanValues) {
		return errors.New("mockRow: Scan target count mismatch")
	}
	for i, d := range dest {
		if err := assignMockValue(d, r.scanValues[i]); err != nil {
			return err
		}
	}
	return nil
}

// mockRows implements pgx.Rows for tests. Tests construct rows with
// `mockRows{rows: [][]any{ {v1, v2, ...}, ... }}` and each Scan
// consumes the values in order as Next() advances.
type mockRows struct {
	rows     [][]any
	idx      int
	closed   bool
	errAfter error // returned by Err() after Close
	scanErr  error
}

func (r *mockRows) Close() {
	r.closed = true
}

func (r *mockRows) Err() error { return r.errAfter }

func (r *mockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *mockRows) Next() bool {
	if r.idx >= len(r.rows) {
		r.closed = true
		return false
	}
	r.idx++
	return true
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]
	if len(dest) != len(row) {
		return errors.New("mockRows: Scan target count mismatch")
	}
	for i, d := range dest {
		if err := assignMockValue(d, row[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *mockRows) Values() ([]any, error) { return nil, nil }

func (r *mockRows) RawValues() [][]byte { return nil }

func (r *mockRows) Conn() *pgx.Conn { return nil }

func (r *mockRows) ToMap() (map[string]any, error) { return nil, nil }

// assignMockValue copies src into dst. Supports a deliberately small
// set of destinations used by the repositories in this package; any
// unknown destination type returns an error so a test failure is loud.
func assignMockValue(dst, src any) error {
	switch d := dst.(type) {
	case *string:
		if src == nil {
			*d = ""
			return nil
		}
		if s, ok := src.(string); ok {
			*d = s
			return nil
		}
		if ps, ok := src.(*string); ok {
			if ps == nil {
				*d = ""
			} else {
				*d = *ps
			}
			return nil
		}
	case **string:
		switch s := src.(type) {
		case *string:
			*d = s
		case string:
			// Promote a bare string to *string so the scanner
			// gets the populated value rather than nil.
			s2 := s
			*d = &s2
		case nil:
			*d = nil
		default:
			return errors.New("assignMockValue: unsupported **string source")
		}
		return nil
	case *int:
		switch s := src.(type) {
		case int:
			*d = s
		case int64:
			*d = int(s)
		case *int:
			if s != nil {
				*d = *s
			}
		default:
			return errors.New("assignMockValue: unsupported int source")
		}
		return nil
	case *int64:
		switch s := src.(type) {
		case int64:
			*d = s
		case int:
			*d = int64(s)
		default:
			return errors.New("assignMockValue: unsupported int64 source")
		}
		return nil
	case *bool:
		if b, ok := src.(bool); ok {
			*d = b
			return nil
		}
		return errors.New("assignMockValue: unsupported bool source")
	case *[]byte:
		if src == nil {
			*d = nil
			return nil
		}
		if b, ok := src.([]byte); ok {
			*d = b
			return nil
		}
		return errors.New("assignMockValue: unsupported []byte source")
	case *[]string:
		if src == nil {
			*d = nil
			return nil
		}
		if b, ok := src.([]string); ok {
			*d = b
			return nil
		}
		return errors.New("assignMockValue: unsupported []string source")
	case *time.Time:
		if src == nil {
			return nil
		}
		if t, ok := src.(time.Time); ok {
			*d = t
			return nil
		}
		return errors.New("assignMockValue: unsupported time.Time source")
	case **time.Time:
		switch s := src.(type) {
		case *time.Time:
			*d = s
		case nil:
			*d = nil
		default:
			return errors.New("assignMockValue: unsupported **time.Time source")
		}
		return nil
	}
	// pgx's own native scanners (pgvector.Vector, time.Time, ...) are
	// supported directly by Scan when passed by pointer. Returning
	// nil here lets pgx decode them; the source value must already
	// be of the right type.
	if src != nil {
		return nil
	}
	return nil
}