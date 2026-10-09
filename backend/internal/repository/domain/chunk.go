package domain

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// Chunk is the AI-KBS chunk domain type. Persisted in the `chunks_ai`
// SQL table (fork-compat; see package doc). Application code uses
// `Chunk`; SQL uses the TableChunks constant.
type Chunk struct {
	ID              string
	DocumentID       string
	VersionNum      int
	Ordinal         int
	Content         string
	ContentMD5      string
	// Embedding is the pgvector vector; nil if not yet embedded. The
	// repository passes it through `pgvector-go` to read/write the
	// pgvector type cleanly.
	Embedding       pgvector.Vector
	EmbeddingModel  string
	CharCount       int
	TokenCount      *int
	Metadata        []byte
	CreatedAt       time.Time
}