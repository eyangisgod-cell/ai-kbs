//go:build cgo

// File-level note: this file holds the CGO-only vector-encoding helpers for
// the sqlite retriever. The underlying sqlite-vec-go-bindings package is
// CGO-only; this file is gated behind `//go:build cgo`. A no-op stub lives
// in vec_stub.go.
//
// TODO(T1.x): remove the stub once a C toolchain is provisioned on every
// developer workstation and CI runner.
package sqlite

import (
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// vecSerializeFloat32 is the CGO-backed float32 vector serializer.
func vecSerializeFloat32(vec []float32) ([]byte, error) {
	return sqlite_vec.SerializeFloat32(vec)
}

// vecAuto is the CGO-backed vector extension auto-registration.
func vecAuto() {
	sqlite_vec.Auto()
}
