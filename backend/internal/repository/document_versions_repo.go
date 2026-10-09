package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableDocumentVersions is the SQL table backing DocumentVersionRepository.
const TableDocumentVersions = "document_versions"

// DocumentVersionRepository persists the per-document version chain.
//
// The full CAS loop (compare base_version, bump, insert new version row)
// lives in `internal/doc/cas.go` (T1.4); this repository exposes the
// primitives that loop composes.
type DocumentVersionRepository interface {
	Create(ctx context.Context, v *domain.DocumentVersion) error
	Get(ctx context.Context, id string) (*domain.DocumentVersion, error)
	GetByVersion(ctx context.Context, documentID string, version int) (*domain.DocumentVersion, error)
	ListByDocument(ctx context.Context, documentID string) ([]*domain.DocumentVersion, error)
	// DeleteByDocumentVersion is the seam used by internal/doc/cas.go
	// (T1.4) to roll back an orphan version row when the document
	// header CAS bump loses a race after the version row was
	// already inserted. Idempotent (returns ErrNotFound when no row
	// matches); operators may also use this for janitor sweeps of
	// stale orphans from a process crash between steps 4 and 5 of
	// the CAS loop.
	DeleteByDocumentVersion(ctx context.Context, documentID string, version int) error
}

type documentVersionRepo struct {
	db DBTX
}

// NewDocumentVersionRepository wires a DocumentVersionRepository to a
// DBTX (pool or tx).
func NewDocumentVersionRepository(db *pgxpool.Pool) DocumentVersionRepository {
	return &documentVersionRepo{db: db}
}

func (r *documentVersionRepo) Create(ctx context.Context, v *domain.DocumentVersion) error {
	if v == nil {
		return errors.New("repository: document version is nil")
	}
	meta := v.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `INSERT INTO document_versions (id, document_id, version_num,
	                                          parent_version, md5, content,
	                                          storage_key, size_bytes, author_id,
	                                          message, base_version, metadata,
	                                          created_at, deleted_flag)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3,
	                   $4, $5, $6,
	                   $7, $8, $9,
	                   $10, $11, $12,
	                   NOW(), FALSE)`
	_, err := r.db.Exec(ctx, q,
		v.ID, v.DocumentID, v.VersionNum,
		v.ParentVersion, v.MD5, v.Content,
		v.StorageKey, v.SizeBytes, v.AuthorID,
		v.Message, v.BaseVersion, meta,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("document_version doc=%s version=%d: %w",
				v.DocumentID, v.VersionNum, mapped)
		}
		return fmt.Errorf("document_version.Create doc=%s: %w", v.DocumentID, err)
	}
	return nil
}

func (r *documentVersionRepo) Get(ctx context.Context, id string) (*domain.DocumentVersion, error) {
	const q = `SELECT id, document_id, version_num, parent_version, md5, content,
	                  storage_key, size_bytes, author_id, message, base_version,
	                  metadata, created_at, deleted_flag
	           FROM document_versions WHERE id = $1 AND deleted_flag = FALSE`
	v, err := scanDocumentVersion(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("document_version.Get id=%s: %w", id, err)
	}
	return v, nil
}

func (r *documentVersionRepo) GetByVersion(ctx context.Context, documentID string, version int) (*domain.DocumentVersion, error) {
	const q = `SELECT id, document_id, version_num, parent_version, md5, content,
	                  storage_key, size_bytes, author_id, message, base_version,
	                  metadata, created_at, deleted_flag
	           FROM document_versions WHERE document_id = $1 AND version_num = $2 AND deleted_flag = FALSE`
	v, err := scanDocumentVersion(r.db.QueryRow(ctx, q, documentID, version))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("document_version.GetByVersion doc=%s v=%d: %w", documentID, version, err)
	}
	return v, nil
}

func (r *documentVersionRepo) ListByDocument(ctx context.Context, documentID string) ([]*domain.DocumentVersion, error) {
	const q = `SELECT id, document_id, version_num, parent_version, md5, content,
	                  storage_key, size_bytes, author_id, message, base_version,
	                  metadata, created_at, deleted_flag
	           FROM document_versions WHERE document_id = $1 AND deleted_flag = FALSE
	           ORDER BY version_num DESC`
	rows, err := r.db.Query(ctx, q, documentID)
	if err != nil {
		return nil, fmt.Errorf("document_version.ListByDocument doc=%s: %w", documentID, err)
	}
	defer rows.Close()
	var out []*domain.DocumentVersion
	for rows.Next() {
		v, err := scanDocumentVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("document_version.ListByDocument rows.Err: %w", err)
	}
	return out, nil
}

// DeleteByDocumentVersion removes a single version row by
// (document_id, version_num). Idempotent: returns ErrNotFound when no
// row matches. The CAS rollback path in internal/doc/cas.go (T1.4)
// depends on ErrNotFound being a non-error signal here so that a
// double-rollback (e.g. operator janitor already swept) does not
// mask the original CAS failure being surfaced upstream.
func (r *documentVersionRepo) DeleteByDocumentVersion(ctx context.Context, documentID string, version int) error {
	const q = `DELETE FROM document_versions
	           WHERE document_id = $1 AND version_num = $2`
	tag, err := r.db.Exec(ctx, q, documentID, version)
	if err != nil {
		return fmt.Errorf("document_version.DeleteByDocumentVersion doc=%s v=%d: %w",
			documentID, version, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDocumentVersion(row pgx.Row) (*domain.DocumentVersion, error) {
	var v domain.DocumentVersion
	var parentVer *int
	var storageKey *string
	err := row.Scan(
		&v.ID, &v.DocumentID, &v.VersionNum, &parentVer, &v.MD5, &v.Content,
		&storageKey, &v.SizeBytes, &v.AuthorID, &v.Message, &v.BaseVersion,
		&v.Metadata, &v.CreatedAt, &v.DeletedFlag,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	v.ParentVersion = parentVer
	if storageKey != nil {
		v.StorageKey = *storageKey
	}
	return &v, nil
}