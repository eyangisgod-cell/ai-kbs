package envelope

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the frozen v1.0 wire format. Adding fields is a breaking change.
//
// Use Ok(data, requestID) on the happy path; use New(code, message, requestID)
// on the failure path. Add structured detail via WithMeta; never edit the
// frozen fields after construction.
type Envelope struct {
	Success   bool           `json:"success"`
	ExitCode  int64          `json:"exit_code"`
	Data      any            `json:"data"`
	Error     *ErrDetail     `json:"error"`
	RequestID string         `json:"request_id"`
	TraceID   string         `json:"trace_id,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
}

// ErrDetail is the failure payload. Code is the wire-format identifier;
// Message is human-readable (may be empty for code-only failures).
type ErrDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// okExitCode is the only legal value when Success is true (design.md §3.1).
const okExitCode int64 = 0

// Ok builds a success envelope with the given payload and request_id.
// Data is required — pass `nil` deliberately only when the contract is "no
// body on success" (rare; e.g. 204 No Content analogues).
func Ok(data any, requestID string) *Envelope {
	return &Envelope{
		Success:   true,
		ExitCode:  okExitCode,
		Data:      data,
		Error:     nil,
		RequestID: requestID,
	}
}

// New builds a failure envelope. The exit_code is derived from MapStatus(code)
// so the wire transport and the body stay in lock-step.
//
// Message may be empty; clients should rely on error.code, not message.
func New(code Code, message, requestID string) *Envelope {
	return &Envelope{
		Success:   false,
		ExitCode:  int64(MapStatus(code)),
		Data:      nil,
		Error:     &ErrDetail{Code: code, Message: message},
		RequestID: requestID,
	}
}

// WithMeta attaches structured detail to env (e.g. base_version for a
// doc.version_conflict). Returns a new envelope; original is untouched so
// callers can safely reuse envelopes in tests.
func WithMeta(env *Envelope, meta map[string]any) *Envelope {
	if env == nil {
		return nil
	}
	cp := *env
	if len(meta) > 0 {
		cp.Meta = make(map[string]any, len(meta))
		for k, v := range meta {
			cp.Meta[k] = v
		}
	}
	return &cp
}

// WithTraceID stamps a W3C trace-id on the envelope (OTLP path only).
func WithTraceID(env *Envelope, traceID string) *Envelope {
	if env == nil || traceID == "" {
		return env
	}
	cp := *env
	cp.TraceID = traceID
	return &cp
}

// WriteJSON serializes env as the response body and stamps the X-Request-ID
// header. status is the HTTP status; on success envelopes it must be 2xx,
// on failure envelopes it must equal MapStatus(env.Error.Code).
func WriteJSON(c *gin.Context, status int, env *Envelope) {
	if env == nil {
		// Defensive default — never return an empty body.
		env = New(CodeInternalError, "empty envelope", "")
	}
	c.Header("X-Request-ID", env.RequestID)
	c.JSON(status, env)
}

// StatusOf returns the HTTP status for the envelope. Falls back to 500 when
// the envelope is nil or its error code is unknown.
func StatusOf(env *Envelope) int {
	if env == nil {
		return http.StatusInternalServerError
	}
	if env.Success {
		return http.StatusOK
	}
	if env.Error != nil {
		return MapStatus(env.Error.Code)
	}
	return http.StatusInternalServerError
}
