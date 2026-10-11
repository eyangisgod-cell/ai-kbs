package domain

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// SAGEvent is a semantic-graph event. Maps to the `sag_events` table.
type SAGEvent struct {
	ID             string
	ChunkID        string
	DocumentID     string
	EventText      string
	// EventEmbedding is the pgvector embedding for the event; nil if
	// not yet embedded.
	EventEmbedding pgvector.Vector
	EventMetadata  []byte
	OccurredAt     time.Time
	CreatedAt      time.Time
}