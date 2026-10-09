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

// TableChunks is the SQL table backing ChunkRepository. Fork-compat:
// the WeKnora fork already owns `chunks`, so the AI-KBS chunks live in
// `chunks_ai`. See package doc.go for the rationale.
const TableChunks = "chunks_ai"

// ChunkRepository persists chunk rows. Persisted in the `chunks_ai`
// SQL table (fork-compat; see package doc).
type ChunkRepository interface {
	Create(ctx context.Context, c *domain.Chunk) error
	Get(ctx context.Context, id string) (*domain.Chunk, error)
	ListByDocumentVersion(ctx context.Context, documentID string, version int) ([]*domain.Chunk, error)
	DeleteByDocumentVersion(ctx context.Context, documentID string, version int) error
}

type chunkRepo struct {
	db DBTX
}

// NewChunkRepository wires a ChunkRepository to a DBTX (pool or tx).
func NewChunkRepository(db *pgxpool.Pool) ChunkRepository { return &chunkRepo{db: db} }

func (r *chunkRepo) Create(ctx context.Context, c *domain.Chunk) error {
	if c == nil {
		return errors.New("repository: chunk is nil")
	}
	meta := c.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	const q = `INSERT INTO chunks_ai (id, document_id, version_num, ordinal,
	                                 content, content_md5, embedding, embedding_model,
	                                 char_count, token_count, metadata, created_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4,
	                   $5, $6, $7, $8,
	                   $9, $10, $11, NOW())`
	_, err := r.db.Exec(ctx, q,
		c.ID, c.DocumentID, c.VersionNum, c.Ordinal,
		c.Content, c.ContentMD5, c.Embedding, c.EmbeddingModel,
		c.CharCount, c.TokenCount, meta,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("chunk ordinal=%d doc=%s: %w", c.Ordinal, c.DocumentID, mapped)
		}
		return fmt.Errorf("chunk.Create ordinal=%d: %w", c.Ordinal, err)
	}
	return nil
}

func (r *chunkRepo) Get(ctx context.Context, id string) (*domain.Chunk, error) {
	const q = `SELECT id, document_id, version_num, ordinal, content, content_md5,
	                  embedding, embedding_model, char_count, token_count,
	                  metadata, created_at
	           FROM chunks_ai WHERE id = $1`
	c, err := scanChunk(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("chunk.Get id=%s: %w", id, err)
	}
	return c, nil
}

func (r *chunkRepo) ListByDocumentVersion(ctx context.Context, documentID string, version int) ([]*domain.Chunk, error) {
	const q = `SELECT id, document_id, version_num, ordinal, content, content_md5,
	                  embedding, embedding_model, char_count, token_count,
	                  metadata, created_at
	           FROM chunks_ai WHERE document_id = $1 AND version_num = $2
	           ORDER BY ordinal ASC`
	rows, err := r.db.Query(ctx, q, documentID, version)
	if err != nil {
		return nil, fmt.Errorf("chunk.ListByDocumentVersion doc=%s v=%d: %w", documentID, version, err)
	}
	defer rows.Close()
	var out []*domain.Chunk
	for rows.Next() {
		c, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chunk.ListByDocumentVersion rows.Err: %w", err)
	}
	return out, nil
}

func (r *chunkRepo) DeleteByDocumentVersion(ctx context.Context, documentID string, version int) error {
	const q = `DELETE FROM chunks_ai WHERE document_id = $1 AND version_num = $2`
	_, err := r.db.Exec(ctx, q, documentID, version)
	if err != nil {
		return fmt.Errorf("chunk.DeleteByDocumentVersion doc=%s v=%d: %w", documentID, version, err)
	}
	return nil
}

func scanChunk(row pgx.Row) (*domain.Chunk, error) {
	var c domain.Chunk
	err := row.Scan(
		&c.ID, &c.DocumentID, &c.VersionNum, &c.Ordinal, &c.Content, &c.ContentMD5,
		&c.Embedding, &c.EmbeddingModel, &c.CharCount, &c.TokenCount,
		&c.Metadata, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}