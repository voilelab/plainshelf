// Package apitest is the harness the API contract suite is written against.
//
// A contract test builds an Env — a started App, its real router, and a shelf
// on a temp directory — issues requests through it, and asserts the status,
// headers and body the API promises. The packages under server/contract hold
// those tests, one per area of the API; everything they share lives here, so a
// helper is either part of this package's documented surface or private to the
// one package that uses it.
//
// url.go is the entry point worth reading first: it names every route
// server/routes.go registers, in the same order.
package apitest
