package repository

import "github.com/jackc/pgx/v5/pgxpool"

// Repositories is the dependency-injection container that holds one
// reference per repository. Use it from `internal/server` (or from the
// test harness) to wire all 13 repositories off a single pgx pool.
//
// Repositories is intentionally a plain struct (not a dig provider) so
// that hand-written tests can instantiate any single repo without
// pulling the entire container.
type Repositories struct {
	pool *pgxpool.Pool

	Workspaces          WorkspaceRepository
	Spaces              SpaceRepository
	Documents           DocumentRepository
	DocumentVersions    DocumentVersionRepository
	Chunks              ChunkRepository
	SagEvents           SagEventRepository
	SagEntities         SagEntityRepository
	SagEventEntityLinks SagEventEntityLinkRepository
	Users               UserRepository
	WorkspaceUsers      WorkspaceUserRepository
	APIKeys             APIKeyRepository
	Sessions            SessionRepository
	AuditLogs           AuditLogRepository
}

// NewRepositories wires the 13 repositories onto a single pgx pool.
//
// The pool MUST be configured (ParseConfig + Ping) by the caller. This
// constructor never closes the pool.
func NewRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		pool:                pool,
		Workspaces:          NewWorkspaceRepository(pool),
		Spaces:              NewSpaceRepository(pool),
		Documents:           NewDocumentRepository(pool),
		DocumentVersions:    NewDocumentVersionRepository(pool),
		Chunks:              NewChunkRepository(pool),
		SagEvents:           NewSagEventRepository(pool),
		SagEntities:         NewSagEntityRepository(pool),
		SagEventEntityLinks: NewSagEventEntityLinkRepository(pool),
		Users:               NewUserRepository(pool),
		WorkspaceUsers:      NewWorkspaceUserRepository(pool),
		APIKeys:             NewAPIKeyRepository(pool),
		Sessions:            NewSessionRepository(pool),
		AuditLogs:           NewAuditLogRepository(pool),
	}
}

// Pool exposes the underlying pgx pool. Use sparingly — repositories
// MUST NOT be re-implemented ad-hoc; extend this package instead.
func (r *Repositories) Pool() *pgxpool.Pool { return r.pool }