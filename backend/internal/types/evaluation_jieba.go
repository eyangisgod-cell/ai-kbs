//go:build cgo

// File-level note: this file holds the gojieba-backed Chinese segmenter.
// gojieba is a CGO-only package (it wraps C++ Jieba sources), so the file is
// gated behind `//go:build cgo`. The pure-Go fallback lives in
// evaluation_stub.go.
//
// TODO(T1.x): remove the stub once a C toolchain is provisioned on every
// developer workstation and CI runner.
package types

import (
	"os"
	"path/filepath"

	"github.com/yanyiwu/gojieba"
)

// Jieba is a global instance of Chinese text segmentation tool
var Jieba *gojieba.Jieba = newJieba()

func newJieba() *gojieba.Jieba {
	dictDir := os.Getenv("JIEBA_DICT_DIR")
	if dictDir == "" {
		return gojieba.NewJieba()
	}

	return gojieba.NewJieba(
		filepath.Join(dictDir, "jieba.dict.utf8"),
		filepath.Join(dictDir, "hmm_model.utf8"),
		filepath.Join(dictDir, "user.dict.utf8"),
		filepath.Join(dictDir, "idf.utf8"),
		filepath.Join(dictDir, "stop_words.utf8"),
	)
}
