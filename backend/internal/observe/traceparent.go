package observe

import "strings"

// Traceparent is the parsed result of a W3C `traceparent` header
// (https://www.w3.org/TR/trace-context/#traceparent-header-values).
//
// Wire format (current version, length 55 chars):
//
//	00-<trace-id-32hex>-<parent-id-16hex>-<flags-2hex>
//
// Only the trace_id and parent span id are needed downstream; flags are
// kept so callers can honour upstream sampling decisions, but the field
// is best treated as "non-authoritative" — many clients always send
// flags=01 regardless of their own internal sampling state.
type Traceparent struct {
	// TraceID is the W3C 32-hex (lowercase) id of the upstream trace. It
	// propagates into envelope v1.0's optional trace_id field (see
	// design.md §3.1) so an operator can correlate an AI-KBS response
	// with the originating call in the trace UI without reading logs.
	TraceID string
	// SpanID is the 16-hex parent span id of the upstream caller. The
	// root span we mint in this service becomes a child of it (the OTel
	// propagator handles that wiring automatically).
	SpanID string
	// Sampled is true when the upstream caller asked us to record the
	// trace (flags=01). We always honour this — flipping a sampled trace
	// to not-sampled would break the trace tree in the UI. A non-sampled
	// upstream (flags=00) is still recorded on our side (so the root
	// span appears) but clients downstream can opt out via sampling.
	Sampled bool
}

// ParseTraceparent parses a W3C `traceparent` header value. Returns a
// zero Traceparent (no TraceID) when the input is missing, malformed,
// or carries a future version we don't yet understand. The parser is
// deliberately strict: silently accepting a malformed header would
// propagate a bogus trace id into every envelope and corrupt the
// Langfuse UI's trace tree.
//
// Strict rules:
//
//   - Version MUST be "00" (current version). Future versions (ff-) are
//     ignored so we don't collide with a spec extension we don't model.
//     See https://www.w3.org/TR/trace-context/#versioning-of-traceparent
//   - TraceID MUST be 32 lowercase-hex chars.
//   - SpanID MUST be 16 lowercase-hex chars.
//   - Flags MUST be 2 lowercase-hex chars.
//   - TraceID MUST NOT be all-zero (W3C §3.2.2.1 — invalid).
//   - SpanID MUST NOT be all-zero (W3C §3.2.2.2 — invalid).
//
// The W3C spec is case-insensitive; we lowercase before validating so
// upstream clients sending uppercase hex still propagate correctly.
func ParseTraceparent(header string) Traceparent {
	h := strings.TrimSpace(header)
	if h == "" {
		return Traceparent{}
	}
	parts := strings.Split(h, "-")
	if len(parts) != 4 {
		return Traceparent{}
	}
	version, traceID, spanID, flags := parts[0], parts[1], parts[2], parts[3]
	if version != "00" {
		// Future version: let the spec-compliant vendor decide, not us.
		return Traceparent{}
	}
	if len(traceID) != 32 || !isLowerHex(traceID) {
		return Traceparent{}
	}
	if len(spanID) != 16 || !isLowerHex(spanID) {
		return Traceparent{}
	}
	if len(flags) != 2 || !isLowerHex(flags) {
		return Traceparent{}
	}
	if isAllZeroHex(traceID) || isAllZeroHex(spanID) {
		// W3C §3.2.2.1/2: all-zero ids are reserved as invalid.
		return Traceparent{}
	}
	return Traceparent{
		TraceID: traceID,
		SpanID:  spanID,
		Sampled: flags == "01",
	}
}

// isLowerHex reports whether s is non-empty and every byte is in
// [0-9a-f]. Uppercase A-F is rejected: W3C calls the encoding
// "lower-hex" and observability tools assume lowercase. Strictness here
// surfaces client bugs in CI rather than after deploy.
func isLowerHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isAllZeroHex reports whether every byte of s is '0'. Used to enforce
// the W3C all-zero reserved-id rule; OTel libraries refuse to import a
// SpanContext built from all-zero ids too.
func isAllZeroHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}
