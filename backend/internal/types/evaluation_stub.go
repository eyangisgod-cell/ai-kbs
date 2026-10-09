//go:build !cgo

// File-level note: this file is the pure-Go stub for the gojieba-backed
// Chinese segmenter when the project is built with CGO_ENABLED=0 (no gcc).
// The full implementation lives in evaluation_jieba.go under `//go:build cgo`.
// The stub returns the input text tokenized on whitespace which is
// semantically safe but degrades Chinese search quality.
//
// TODO(T1.x): remove this stub once a C toolchain is provisioned on every
// developer workstation and CI runner.
package types

import (
	"os"
	"strings"
)

// stubJieba is the no-CGO shim. Cut / CutForSearch fall through to a
// whitespace tokenizer; Chinese word segmentation is unavailable until CGO is
// enabled and the real gojieba package is linked in.
type stubJieba struct{}

// Jieba is the package-level tokenizer instance.
var Jieba = &stubJieba{}

// Cut returns the tokens for the input text. The stub splits on whitespace;
// Chinese text is returned as a single token. Set JIEBA_DICT_DIR to enable
// the CGO-backed implementation.
func (s *stubJieba) Cut(text string, _ bool) []string {
	return splitOnWhitespace(text)
}

// CutAll returns the tokens for the input text using full-pattern matching.
// The stub returns the same output as Cut.
func (s *stubJieba) CutAll(text string) []string {
	return splitOnWhitespace(text)
}

// CutForSearch returns the tokens optimized for search-indexing. The stub
// splits on whitespace and lowercases ASCII tokens.
func (s *stubJieba) CutForSearch(text string, _ bool) []string {
	return splitOnWhitespace(text)
}

// newJieba is the stub constructor; the real constructor in
// evaluation_jieba.go (CGO) honors JIEBA_DICT_DIR.
func newJieba() *stubJieba {
	_ = os.Getenv("JIEBA_DICT_DIR") // env var has no effect in stub
	return &stubJieba{}
}

// splitOnWhitespace is the fallback tokenizer used by the stub.
func splitOnWhitespace(text string) []string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = strings.ToLower(f)
	}
	return out
}
