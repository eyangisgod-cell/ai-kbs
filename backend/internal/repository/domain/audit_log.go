package domain

import "time"

// AuditActorType enumerates the actor classes recorded on an audit log.
type AuditActorType string

const (
	AuditActorUser    AuditActorType = "user"
	AuditActorAPIKey  AuditActorType = "api_key"
	AuditActorSystem  AuditActorType = "system"
)

// AuditLog is the immutable audit record. Persisted in the
// `audit_logs_ai` SQL table (fork-compat; see package doc).
//
// The repository NEVER logs the payload field — it may carry tokens,
// IP+UA combinations, or PII. The payload is only surfaced to the
// operator log at the handler edge, after envelope filtering.
type AuditLog struct {
	ID           string
	WorkspaceID  string
	ActorID      string
	ActorType    AuditActorType
	Action       string
	TargetType   string
	TargetID     string
	RequestID    string
	TraceID      string
	IP           string
	UserAgent    string
	// Payload is JSONB with arbitrary operator-defined keys. Surface
	// as json.RawMessage so callers decide on a deserializer.
	Payload     []byte
	CreatedAt   time.Time
}