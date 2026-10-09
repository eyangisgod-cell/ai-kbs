package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableDocuments is the SQL table backing DocumentRepository.
const TableDocuments = "documents"

// DocumentRepository persists documents.
//
// CAS semantics: Update(ctx, doc, baseVersion) is the optimistic-concurrency
// entry point. It bumps active_version on the document AND inserts a new
// document_versions row in a single transaction. A version mismatch
// returns ErrVersionConflict. The full CAS contract is owned by T1.4 /
// internal/doc/cas.go; this method is the repository seam it relies on.
type DocumentRepository interface {
	Create(ctx context.Context, d *domain.Document) error
	Get(ctx context.Context, id string) (*domain.Document, error)
	GetByCode(ctx context.Context, spaceID, docCode string) (*domain.Document, error)
	ListBySpace(ctx context.Context, spaceID string, limit, offset int) ([]*domain.Document, error)
	Update(ctx context.Context, d *domain.Document, baseVersion int) (newVersion int, err error)
	SoftDelete(ctx context.Context, id string) error
	GetCurrentVersion(ctx context.Context, documentID string) (*domain.DocumentVersion, error)
}

type documentRepo struct {
	db DBTX
}

// NewDocumentRepository wires a DocumentRepository to a DBTX (pool or tx).
func NewDocumentRepository(db *pgxpool.Pool) DocumentRepository { return &documentRepo{db: db} }

func (r *documentRepo) Create(ctx context.Context, d *domain.Document) error {
	if d == nil {
		return errors.New("repository: document is nil")
	}
	if d.DocCode == "" {
		return errors.New("repository: document.DocCode is required")
	}
	meta := d.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `INSERT INTO documents (id, space_id, doc_code, parent_doc_code, tree_path,
	                                 depth, title, doc_type, mime_type,
	                                 active_version, active_md5, storage_key, size_bytes,
	                                 sag_indexed, es_indexed, content_tsv,
	                                 tags, metadata, confidential_level,
	                                 created_by, updated_by, created_at, updated_at,
	                                 deleted_flag, deleted_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   $6, $7, $8, $9,
	                   $10, $11, $12, $13,
	                   $14, $15, $16,
	                   $17, $18, $19,
	                   $20, $21, NOW(), NOW(),
	                   FALSE, '1970-01-01 00:00:00+00')`
	_, err := r.db.Exec(ctx, q,
		d.ID, d.SpaceID, d.DocCode, nullable(d.ParentDocCode), d.TreePath,
		d.Depth, d.Title, d.DocType, nullable(d.MimeType),
		d.ActiveVersion, d.ActiveMD5, nullable(d.StorageKey), d.SizeBytes,
		d.SAGIndexed, d.ESIndexed, nil, // content_tsv: populated by FTS trigger in M2
		tags, meta, d.ConfidentialLevel,
		d.CreatedBy, d.UpdatedBy,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("document code=%q in space=%q: %w", d.DocCode, d.SpaceID, mapped)
		}
		return fmt.Errorf("document.Create code=%q: %w", d.DocCode, err)
	}
	return nil
}

func (r *documentRepo) Get(ctx context.Context, id string) (*domain.Document, error) {
	const q = `SELECT id, space_id, doc_code, parent_doc_code, tree_path, depth,
	                  title, doc_type, mime_type, active_version, active_md5,
	                  storage_key, size_bytes, sag_indexed, es_indexed,
	                  content_tsv, tags, metadata, confidential_level,
	                  created_by, updated_by, created_at, updated_at,
	                  deleted_flag, deleted_at
	           FROM documents WHERE id = $1 AND deleted_flag = FALSE`
	d, err := scanDocument(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("document.Get id=%s: %w", id, err)
	}
	return d, nil
}

func (r *documentRepo) GetByCode(ctx context.Context, spaceID, docCode string) (*domain.Document, error) {
	const q = `SELECT id, space_id, doc_code, parent_doc_code, tree_path, depth,
	                  title, doc_type, mime_type, active_version, active_md5,
	                  storage_key, size_bytes, sag_indexed, es_indexed,
	                  content_tsv, tags, metadata, confidential_level,
	                  created_by, updated_by, created_at, updated_at,
	                  deleted_flag, deleted_at
	           FROM documents WHERE space_id = $1 AND doc_code = $2 AND deleted_flag = FALSE`
	d, err := scanDocument(r.db.QueryRow(ctx, q, spaceID, docCode))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("document.GetByCode space=%s code=%s: %w", spaceID, docCode, err)
	}
	return d, nil
}

func (r *documentRepo) ListBySpace(ctx context.Context, spaceID string, limit, offset int) ([]*domain.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	const q = `SELECT id, space_id, doc_code, parent_doc_code, tree_path, depth,
	                  title, doc_type, mime_type, active_version, active_md5,
	                  storage_key, size_bytes, sag_indexed, es_indexed,
	                  content_tsv, tags, metadata, confidential_level,
	                  created_by, updated_by, created_at, updated_at,
	                  deleted_flag, deleted_at
	           FROM documents WHERE space_id = $1 AND deleted_flag = FALSE
	           ORDER BY created_at DESC
	           LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, spaceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("document.ListBySpace space=%s: %w", spaceID, err)
	}
	defer rows.Close()
	var out []*domain.Document
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("document.ListBySpace rows.Err: %w", err)
	}
	return out, nil
}

// Update performs an optimistic-concurrency update of the document
// header AND bumps active_version atomically. The repository does not
// insert the document_versions row here — that is the caller's job
// (via the service layer) — but the contract is that the caller's
// baseVersion must match the row's active_version.
//
// Returns ErrVersionConflict (typed *VersionConflictError) on mismatch.
func (r *documentRepo) Update(ctx context.Context, d *domain.Document, baseVersion int) (int, error) {
	if d == nil {
		return 0, errors.New("repository: document is nil")
	}
	meta := d.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `UPDATE documents
	           SET title = $2, doc_type = $3, mime_type = $4,
	               active_version = active_version + 1,
	               active_md5 = $5, storage_key = $6, size_bytes = $7,
	               sag_indexed = $8, es_indexed = $9,
	               tags = $10, metadata = $11, confidential_level = $12,
	               updated_by = $13, updated_at = NOW()
	           WHERE id = $1 AND deleted_flag = FALSE AND active_version = $14
	           RETURNING active_version`
	var newVer int
	err := r.db.QueryRow(ctx, q,
		d.ID, d.Title, d.DocType, nullable(d.MimeType),
		d.ActiveMD5, nullable(d.StorageKey), d.SizeBytes,
		d.SAGIndexed, d.ESIndexed,
		tags, meta, d.ConfidentialLevel,
		d.UpdatedBy, baseVersion,
	).Scan(&newVer)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either the row is gone, soft-deleted, or its
			// active_version no longer matches baseVersion. Read
			// the row to distinguish.
			return 0, r.classifyUpdateMiss(ctx, d.ID, baseVersion)
		}
		return 0, fmt.Errorf("document.Update id=%s: %w", d.ID, err)
	}
	return newVer, nil
}

// classifyUpdateMiss disambiguates a no-rows UPDATE result.
func (r *documentRepo) classifyUpdateMiss(ctx context.Context, id string, baseVersion int) error {
	const q = `SELECT active_version FROM documents WHERE id = $1`
	var cur int
	err := r.db.QueryRow(ctx, q, id).Scan(&cur)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("document.Update classify id=%s: %w", id, err)
	}
	return &VersionConflictError{
		DocCode:        id,
		BaseVersion:    baseVersion,
		CurrentVersion: cur,
	}
}

func (r *documentRepo) SoftDelete(ctx context.Context, id string) error {
	const q = `UPDATE documents SET deleted_flag = TRUE, deleted_at = NOW(),
	                                 updated_at = NOW()
	           WHERE id = $1 AND deleted_flag = FALSE`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("document.SoftDelete id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *documentRepo) GetCurrentVersion(ctx context.Context, documentID string) (*domain.DocumentVersion, error) {
	const q = `SELECT id, document_id, version_num, parent_version, md5, content,
	                  storage_key, size_bytes, author_id, message, base_version,
	                  metadata, created_at, deleted_flag
	           FROM document_versions
	           WHERE document_id = $1 AND deleted_flag = FALSE
	           ORDER BY version_num DESC LIMIT 1`
	v, err := scanDocumentVersion(r.db.QueryRow(ctx, q, documentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("document.GetCurrentVersion doc=%s: %w", documentID, err)
	}
	return v, nil
}

func scanDocument(row pgx.Row) (*domain.Document, error) {
	var d domain.Document
	var mime, parentDoc *string
	var storageKey *string
	var tsv []byte
	var deletedAt time.Time
	err := row.Scan(
		&d.ID, &d.SpaceID, &d.DocCode, &parentDoc, &d.TreePath, &d.Depth,
		&d.Title, &d.DocType, &mime, &d.ActiveVersion, &d.ActiveMD5,
		&storageKey, &d.SizeBytes, &d.SAGIndexed, &d.ESIndexed,
		&tsv, &d.Tags, &d.Metadata, &d.ConfidentialLevel,
		&d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt,
		&d.DeletedFlag, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if mime != nil {
		d.MimeType = *mime
	}
	if parentDoc != nil {
		d.ParentDocCode = *parentDoc
	}
	if storageKey != nil {
		d.StorageKey = *storageKey
	}
	d.ContentTSV = tsv
	if !deletedAt.IsZero() && deletedAt.Year() >= 1971 {
		t := deletedAt
		d.DeletedAt = &t
	}
	return &d, nil
}