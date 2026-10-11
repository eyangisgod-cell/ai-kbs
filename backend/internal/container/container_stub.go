//go:build !cgo

// File-level note: this file is the pure-Go twin of container_cgo.go for
// builds with CGO_ENABLED=0. The stub does not register the vec0 SQLite
// extension and does not register the DuckDB driver; both capabilities are
// gated behind a CGO build until a C toolchain is provisioned on every
// developer workstation and CI runner (T1.x).
package container

// vecAuto is the no-CGO shim. SQLite vector search is unavailable in this
// build; the production deployments MUST set CGO_ENABLED=1.
func vecAuto() {}

// registerDuckdb is a no-op in non-CGO builds. The DuckDB driver is not
// registered; SQL queries against the in-memory DuckDB engine will fail.
func registerDuckdb() {}
