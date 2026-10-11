package domain

import "time"

// ChunkStrategy enumerates supported chunk-splitting algorithms.
type ChunkStrategy string

const (
	ChunkStrategySemantic     ChunkStrategy = "semantic"
	ChunkStrategyParentChild  ChunkStrategy = "parent_child"
	ChunkStrategyFixed        ChunkStrategy = "fixed"
)

// Space is the AI-KBS knowledge-base equivalent of the fork's
// KnowledgeBase. Maps to the `spaces` table (migration 000079).
type Space struct {
	ID             string
	WorkspaceID    string
	Code           string
	Name           string
	Description    string
	OwnerID        string
	EmbeddingModel string
	EmbeddingDim   int
	ChunkStrategy  ChunkStrategy
	ChunkSize      int
	ChunkOverlap   int
	SAGEnabled     bool
	// Metadata is JSONB. Repository code stores it as a json.RawMessage
	// so callers can decide how to deserialize.
	Metadata      []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// DeletedAt holds the soft-delete sentinel. The schema uses
	// `deleted_at >= '2100-01-01'` to mark live rows; the repository
	// surfaces that as nil for live rows and a non-nil time for trashed
	// rows.
	DeletedAt *time.Time
}

// IsDeleted reports whether the space has been soft-deleted.
func (s *Space) IsDeleted() bool { return s != nil && s.DeletedAt != nil }