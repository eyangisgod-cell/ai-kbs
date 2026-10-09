package domain

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// SAGEntity is a node in the semantic-actor graph. Maps to the
// `sag_entities` table.
type SAGEntity struct {
	ID          string
	SpaceID     string
	Name        string
	EntityType  string
	Description string
	// CanonicalID is the id of the canonical entity this one folds
	// into; nil for a canonical row itself.
	CanonicalID *string
	Aliases     []string
	// Embedding is the pgvector embedding; nil if not yet embedded.
	Embedding  pgvector.Vector
	CreatedAt  time.Time
}