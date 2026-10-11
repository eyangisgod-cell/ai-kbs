package envelope

// CodedError is the interface every translateable error satisfies.
//
//   - Code()   → the wire-format namespace.subname (e.g. "doc.version_conflict").
//   - Status() → the HTTP status the handler should write.
//
// The translator in translate.go uses errors.As to extract a *CodedError and
// read these fields; everything else falls through to internal.error.
type CodedError interface {
	error
	Code() string
	Status() int
}

// IsCoded reports whether err (or any error it wraps) implements CodedError.
func IsCoded(err error) bool {
	if err == nil {
		return false
	}
	ce, ok := err.(CodedError)
	if ok {
		return ce.Code() != ""
	}
	// Walk one wrap level — handlers usually own one fmt.Errorf("...: %w", err).
	type unwrapper interface{ Unwrap() error }
	for cur := err; cur != nil; {
		u, ok := cur.(unwrapper)
		if !ok {
			break
		}
		cur = u.Unwrap()
		if cur == nil {
			break
		}
		if ce, ok := cur.(CodedError); ok {
			return ce.Code() != ""
		}
	}
	return false
}
