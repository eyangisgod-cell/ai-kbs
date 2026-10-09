//go:build !cgo

// File-level note: this file is the pure-Go twin of vec_cgo.go for builds
// with CGO_ENABLED=0. SQLite vector search is unavailable in this build;
// the production deployments MUST set CGO_ENABLED=1.
package sqlite

import "errors"

// vecSerializeFloat32 returns an error in non-CGO builds.
func vecSerializeFloat32(_ []float32) ([]byte, error) {
	return nil, errors.New("vecSerializeFloat32 requires CGO build; rebuild with CGO_ENABLED=1")
}
