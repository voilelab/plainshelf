// Package repocheck holds repository-wide invariants that are cheap to state
// and easy to break by accident. It has no runtime code: every check lives in a
// test so `go test ./...` — the gate CI already runs — enforces it.
package repocheck
