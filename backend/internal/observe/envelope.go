package observe

import (
	"github.com/eyangisgod-cell/ai-kbs/internal/errors/envelope"
)

// NewEnvelopeWithTraceID is a convenience builder that mints a v1.0
// envelope with both the request id (required) and an optional W3C
// trace_id. Pass the raw `traceparent` header value (or "" when absent);
// internally this goes through ParseTraceparent so any malformed input
// safely degrades to "no trace_id" rather than writing a bogus 32-hex
// string into the envelope body.
//
// Handlers that build their own envelopes (e.g. happy-path responses
// wrapped manually with envelope.Ok) can skip this and just call
// envelope.WithTraceID(env, parsedTraceID) instead — this helper is for
// the much more common "Ok(data, requestID)" pattern where the trace
// id is implicit in the request context.
//
// design.md §3.1 freezes the envelope schema; trace_id is one of the
// optional top-level fields and is otherwise governed by the W3C
// tracecontext spec (32 lowercase hex chars).
func NewEnvelopeWithTraceID(requestID, traceparentHeader string) *envelope.Envelope {
	env := envelope.Ok(nil, requestID)
	if parsed := ParseTraceparent(traceparentHeader); parsed.TraceID != "" {
		return envelope.WithTraceID(env, parsed.TraceID)
	}
	return env
}

// StampTraceIDFromHeader is the helper used by handlers writing a
// error envelope after c.Error(...) has been recorded by the error
// middleware. They already have the envelope object; this returns the
// stamped copy without them having to import internal/errors/envelope
// directly. Equivalent to envelope.WithTraceID(env, ParseTraceparent
// (traceparentHeader).TraceID).
func StampTraceIDFromHeader(env *envelope.Envelope, traceparentHeader string) *envelope.Envelope {
	if env == nil {
		return nil
	}
	parsed := ParseTraceparent(traceparentHeader)
	if parsed.TraceID == "" {
		return env
	}
	return envelope.WithTraceID(env, parsed.TraceID)
}
