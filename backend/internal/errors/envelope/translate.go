package envelope

import (
	"context"
	"errors"
)

// FromError converts a Go error into an envelope.
//
//   - err == nil                          → Ok(nil, requestID)
//   - errors.As(err, &CodedError)         → New(code.Code(), code.Error(), requestID)
//   - errors.Is(err, context.Deadline...) → New(CodeInternalTimeout, "deadline exceeded", requestID)
//   - otherwise                           → New(CodeInternalError, "internal error", requestID)
//
// Handlers MUST use FromError rather than rolling their own status mapping.
// The translator deliberately strips the original message from the body when
// the error is not CodedError, to avoid leaking SQL / stack detail to clients;
// the full chain lives in logs + spans, keyed by request_id.
func FromError(err error, requestID string) *Envelope {
	if err == nil {
		return Ok(nil, requestID)
	}

	var ce CodedError
	if errors.As(err, &ce) {
		return New(Code(ce.Code()), ce.Error(), requestID)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return New(CodeInternalTimeout, "deadline exceeded", requestID)
	}

	// Untyped error → collapse to internal.error with a generic message.
	// The original chain is preserved server-side via the returned error;
	// the caller is expected to log it alongside the request_id before
	// writing the response.
	return New(CodeInternalError, "internal error", requestID)
}
