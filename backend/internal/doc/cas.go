// Package doc hosts document-domain business logic that sits above the
// repository layer: docCode generation (T1.3) and compare-and-swap
// write coordination (T1.4). The CAS service composes two repositories
// (Document, DocumentVersion) into the atomic "compare base_version,
// insert new version, bump active_version" loop the handler layer
// relies on. It returns repository-defined typed errors
// (*VersionConflictError, ErrNotFound) so handlers can match on the
// sentinel without importing this package.
package doc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository"
	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/google/uuid"
)

// CASService exposes the high-level compare-and-swap write operations
// on documents. T1.4 ships exactly one entry point — UpdateWithCAS —
// because the CLI push / handler PUT both need it; subsequent
// operations (move, resolve) compose against the same primitives.
type CASService interface {
	// UpdateWithCAS performs an optimistic-concurrency write on the
	// document identified by (spaceID, docCode). The caller supplies
	// baseVersion = the version they last saw. If the document has
	// moved on, returns *repository.VersionConflictError (matching
	// errors.Is(ErrVersionConflict)) so the caller can decide
	// whether to retry, render a diff, or escalate to the user.
	//
	// On success, returns the newly inserted DocumentVersion with
	// VersionNum = baseVersion+1 and ParentVersion = baseVersion.
	UpdateWithCAS(ctx context.Context, spaceID, docCode string, baseVersion int, newContent, message, authorID string) (*domain.DocumentVersion, error)
}

type casService struct {
	docs     repository.DocumentRepository
	versions repository.DocumentVersionRepository
}

// NewCASService wires the two repositories needed for the CAS loop.
// Both arguments are required; nil will panic at construction time
// (intentional — DI graph is the only place to instantiate).
func NewCASService(docs repository.DocumentRepository, versions repository.DocumentVersionRepository) CASService {
	if docs == nil {
		panic("doc.NewCASService: docs is nil")
	}
	if versions == nil {
		panic("doc.NewCASService: versions is nil")
	}
	return &casService{docs: docs, versions: versions}
}

// UpdateWithCAS performs the CAS write loop. The contract is:
//
//  1. Reject negative baseVersion (caller bug; non-recoverable).
//  2. Read the current document header by (spaceID, docCode). If its
//     active_version != baseVersion, return *VersionConflictError
//     WITHOUT touching any other row — this is the cheap path for
//     stale clients.
//  3. Build the new DocumentVersion row (SHA-256 hash + UUID id +
//     version = baseVersion+1 + parent_version = baseVersion).
//  4. Insert the version row.
//  5. Atomically bump the document header (UPDATE … WHERE
//     active_version = baseVersion). On conflict here, return
//     *VersionConflictError after best-effort deleting the version
//     row we just inserted.
//  6. Return the new DocumentVersion on success.
//
// The loop is intentionally NOT wrapped in a single SQL transaction
// because the repository layer (T1.2) deliberately split inserts from
// the conditional UPDATE so the repository can run inside its
// caller's tx without an extra abstraction. The orphan-version
// rollback in step 5 is best-effort; on a process crash between steps
// 4 and 5, a janitor sweep reaps version rows whose document_id's
// active_version has not caught up. (See design.md §4.2 — CAS retry
// chain.)
func (s *casService) UpdateWithCAS(ctx context.Context, spaceID, docCode string, baseVersion int, newContent, message, authorID string) (*domain.DocumentVersion, error) {
	if baseVersion < 0 {
		return nil, fmt.Errorf("doc: invalid baseVersion=%d (must be >= 0): %w", baseVersion, errInvalidBaseVersion)
	}
	if docCode == "" {
		return nil, fmt.Errorf("doc: docCode is required: %w", errInvalidBaseVersion)
	}

	// Step 2: read current header.
	current, err := s.docs.GetByCode(ctx, spaceID, docCode)
	if err != nil {
		// Wrap so callers can still errors.Is(err, repository.ErrNotFound)
		// at the boundary. We add the docCode for forensic value.
		return nil, fmt.Errorf("doc: read current space=%s code=%s: %w", spaceID, docCode, err)
	}
	if current.ActiveVersion != baseVersion {
		return nil, &repository.VersionConflictError{
			DocCode:        docCode,
			BaseVersion:    baseVersion,
			CurrentVersion: current.ActiveVersion,
		}
	}

	// Step 3: build the new version row.
	contentHash := sha256.Sum256([]byte(newContent))
	newVersionNum := current.ActiveVersion + 1
	parent := current.ActiveVersion
	newVersion := domain.DocumentVersion{
		ID:            uuid.NewString(),
		DocumentID:    current.ID,
		VersionNum:    newVersionNum,
		ParentVersion: &parent,
		MD5:           hex.EncodeToString(contentHash[:]),
		Content:       newContent,
		SizeBytes:     int64(len(newContent)),
		AuthorID:      authorID,
		Message:       message,
		BaseVersion:   &baseVersion,
		CreatedAt:     time.Now().UTC(),
		DeletedFlag:   false,
	}

	// Step 4: insert the new version row.
	if err := s.versions.Create(ctx, &newVersion); err != nil {
		// On a UNIQUE(document_id, version_num) violation, another
		// writer raced past GetByCode at the same baseVersion and
		// stamped this version number first. Translate to a CAS
		// conflict so the caller can retry at the new head rather
		// than be confused by a generic ErrAlreadyExists. The DB's
		// current_version at this point is at least newVersionNum,
		// so we report CurrentVersion = newVersionNum as a best
		// estimate (the caller should always re-read; design.md
		// §4.2 caps retries at 3).
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, &repository.VersionConflictError{
				DocCode:        docCode,
				BaseVersion:    baseVersion,
				CurrentVersion: newVersionNum,
			}
		}
		return nil, fmt.Errorf("doc: insert version doc=%s v=%d: %w", current.ID, newVersionNum, err)
	}

	// Step 5: bump the header atomically with CAS.
	update := &domain.Document{
		ID:            current.ID,
		SpaceID:       current.SpaceID,
		DocCode:       current.DocCode,
		Title:         current.Title,
		DocType:       current.DocType,
		MimeType:      current.MimeType,
		ActiveVersion: newVersionNum,
		ActiveMD5:     newVersion.MD5,
		Tags:          current.Tags,
		Metadata:      current.Metadata,
		ConfidentialLevel: current.ConfidentialLevel,
		UpdatedBy:     authorID,
		// Required fields are not mutated by the UPDATE clause but
		// may be reassigned to existing values; see DocumentRepository.Update.
	}
	newVer, err := s.docs.Update(ctx, update, baseVersion)
	if err != nil {
		// Best-effort rollback of the version row we just inserted.
		// We deliberately ignore the rollback's error: the original
		// CAS failure is the one the caller needs to see. The orphan
		// version row will be reaped by the janitor sweep.
		if rbErr := s.versions.DeleteByDocumentVersion(ctx, current.ID, newVersionNum); rbErr != nil && !errors.Is(rbErr, repository.ErrNotFound) {
			// surface nothing — this path is best-effort
			_ = rbErr
		}
		return nil, err // *VersionConflictError or other; caller inspects
	}

	// Step 6: succeed. Patch the returned VersionNum from the
	// repository's authoritative bump (it may have lost a race and
	// be > newVersionNum; in that case the *VersionConflictError
	// branch above would have fired first).
	newVersion.VersionNum = newVer
	return &newVersion, nil
}

// errInvalidBaseVersion is the typed sentinel for the
// "caller passed nonsense baseVersion" precondition. Not exported —
// external callers should not retry; a negative baseVersion is a
// client bug and the handler maps it to 400 (envelope
// "doc.invalid_request") when wrapping. We keep a sentinel so
// internal tests can errors.Is/As without coupling to strings.
var errInvalidBaseVersion = errors.New("doc: invalid baseVersion")
