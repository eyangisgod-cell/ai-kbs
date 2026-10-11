//go:build cgo

// File-level note: this file holds the CGO-only bits of the container
// package. The imports sqlite-vec-go-bindings/cgo and duckdb-go (the
// in-memory DuckDB engine) are both CGO-only packages. They are isolated
// here behind `//go:build cgo` so that builds with CGO_ENABLED=0 skip this
// file entirely; a no-op pure-Go twin (vecAuto / registerDuckdb) lives in
// container_stub.go.
//
// TODO(T1.x): remove the stub once a C toolchain is provisioned on every
// developer workstation and CI runner.
package container

import (
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/duckdb/duckdb-go/v2"
)

// vecAuto is the CGO-backed vector store auto-registration. The pure-Go
// stub is a no-op; it does not register the vec0 SQLite extension. SQLite
// vector search therefore requires CGO_ENABLED=1 at build time.
func vecAuto() {
	sqlite_vec.Auto()
}

// registerDuckdb is a side-effect import so that database/sql registers
// the DuckDB driver. The pure-Go stub is a no-op (no driver is registered).
func registerDuckdb() {
	// The blank import above already does the work.
}
