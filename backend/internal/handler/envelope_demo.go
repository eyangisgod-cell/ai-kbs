package handler

import (
	"errors"
	"net/http"

	envelope "github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
	"github.com/eyangisgod-cell/ai-kbs/internal/types"
	"github.com/gin-gonic/gin"
)

// EnvelopeDemoHandler is the reference implementation of the envelope v1.0
// contract (design.md §3.1 / AC-B7).
//
// The endpoint deliberately exercises every branch — success, CodedError
// translation, unknown error collapse, validation, and meta attachment —
// so reviewers can see the canonical pattern. Subsequent handlers should
// follow the same shape; per-instruction the rest of the handler migration
// to envelope lives in follow-up commits, not this one.
type EnvelopeDemoHandler struct{}

// NewEnvelopeDemoHandler constructs the demo handler. No deps required.
func NewEnvelopeDemoHandler() *EnvelopeDemoHandler { return &EnvelopeDemoHandler{} }

// EnvelopeDemoResponse is the data payload for the happy path.
type EnvelopeDemoResponse struct {
	Ping string `json:"ping"`
}

// demoCoded is a fake typed error to demonstrate FromError's CodedError branch.
type demoCoded struct{}

func (demoCoded) Error() string { return "demo coded failure" }
func (demoCoded) Code() string  { return envelope.CodeRBACPermissionDenied }
func (demoCoded) Status() int   { return http.StatusForbidden }

// Demo godoc
// @Summary      Envelope v1.0 contract demo
// @Description  Returns an envelope v1.0 response. Reads the `mode` query
// @Description  parameter to switch branches: `ok` | `coded` | `unknown` |
// @Description  `validation` | `meta`. This handler exists only to make the
// @Description  envelope contract testable end-to-end.
// @Tags         envelope-demo
// @Produce      json
// @Param        mode   query     string  false  "branch selector"  Enums(ok, coded, unknown, validation, meta)
// @Success      200    {object}  envelope.Envelope
// @Failure      400    {object}  envelope.Envelope
// @Failure      403    {object}  envelope.Envelope
// @Failure      500    {object}  envelope.Envelope
// @Router       /envelope-demo [get]
func (h *EnvelopeDemoHandler) Demo(c *gin.Context) {
	rid, _ := c.Get(types.RequestIDContextKey.String())
	ridStr, _ := rid.(string)

	switch c.Query("mode") {
	case "coded":
		// Translate a CodedError into the failure envelope.
		env := envelope.FromError(demoCoded{}, ridStr)
		envelope.WriteJSON(c, envelope.StatusOf(env), env)
		return
	case "unknown":
		// Untyped error collapses to internal.error with a generic message —
		// the original error is logged server-side, not echoed in the body.
		env := envelope.FromError(errors.New("pq: duplicate key value"), ridStr)
		envelope.WriteJSON(c, envelope.StatusOf(env), env)
		return
	case "validation":
		// Hand-crafted validation failure using New + MapStatus.
		env := envelope.New(envelope.CodeDocCodeMalformed, "docCode must be 8-char base62", ridStr)
		envelope.WriteJSON(c, envelope.StatusOf(env), env)
		return
	case "meta":
		// Failure envelope with structured detail via WithMeta.
		env := envelope.New(envelope.CodeDocVersionConflict, "base_version stale", ridStr)
		env = envelope.WithMeta(env, map[string]any{
			"base_version":    int64(3),
			"current_version": int64(9),
		})
		envelope.WriteJSON(c, envelope.StatusOf(env), env)
		return
	default:
		// Happy path.
		env := envelope.Ok(EnvelopeDemoResponse{Ping: "pong"}, ridStr)
		envelope.WriteJSON(c, http.StatusOK, env)
	}
}
