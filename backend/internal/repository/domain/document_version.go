package domain

import "time"

// DocumentVersion is an immutable snapshot of a document at a point in
// time. Maps to the `document_versions` table (migration 000079). The
// CAS base_version reference is preserved here; T1.4 owns the
// optimistic-concurrency semantics on top.
type DocumentVersion struct {
	ID             string
	DocumentID     string
	VersionNum     int
	ParentVersion  *int
	MD5            string
	Content        string
	StorageKey     string
	SizeBytes      int64
	AuthorID       string
	Message        string
	BaseVersion    *int
	Metadata       []byte
	CreatedAt      time.Time
	DeletedFlag    bool
}