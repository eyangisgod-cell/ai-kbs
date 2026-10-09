package domain

import "time"

// Document is the AI-KBS document aggregate. Maps to the `documents`
// table (migration 000079). Parent-child tree structure is encoded via
// (parent_doc_code, tree_path, depth) — these are set by the service
// layer, not the repository.
type Document struct {
	ID                 string
	SpaceID            string
	DocCode            string
	ParentDocCode      string
	TreePath           string
	Depth              int
	Title              string
	DocType            string
	MimeType           string
	ActiveVersion      int
	ActiveMD5          string
	StorageKey         string
	SizeBytes          int64
	SAGIndexed         bool
	ESIndexed          bool
	// ContentTSV is the FTS vector (TSVECTOR). The repository treats it
	// as an opaque []byte when reading; handlers should never expose it.
	ContentTSV []byte
	// Tags is TEXT[]; surfaced as []string.
	Tags []string
	// Metadata is JSONB; surfaced as json.RawMessage.
	Metadata             []byte
	ConfidentialLevel    string
	CreatedBy            string
	UpdatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedFlag          bool
	DeletedAt            *time.Time
}

// IsDeleted reports whether the document has been soft-deleted.
func (d *Document) IsDeleted() bool { return d != nil && d.DeletedAt != nil && !d.DeletedAt.IsZero() && d.DeletedFlag }