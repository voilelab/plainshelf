package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Two gates decide whether a request is a write: the token requirement in
// security.go and the read-only rejection in Handler. Enumerating the methods
// against both makes a reintroduced copy show up as a disagreement rather than
// as silently weaker protection.

var mutatingMethods = []string{
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

var nonMutatingMethods = []string{
	http.MethodGet,
	http.MethodHead,
}

// The read-only gate runs before routing, so the path only has to be under
// /api/ for the token gate to also apply to it.
const methodGateTestPath = "/api/shelves/default_shelf/layers/blocked"

func TestReadOnlyModeRejectsExactlyTheMutatingMethods(t *testing.T) {
	env := newAPITestEnv(t)
	env.app.conf.ReadOnly = true

	for _, method := range mutatingMethods {
		t.Run("rejects "+method, func(t *testing.T) {
			rec := env.do(httptest.NewRequest(method, methodGateTestPath, nil))

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}

	for _, method := range nonMutatingMethods {
		t.Run("allows "+method, func(t *testing.T) {
			rec := env.do(httptest.NewRequest(method, "/api/mode", nil))

			if rec.Code == http.StatusForbidden {
				t.Fatalf("status = %d, want the read-only gate not to fire", rec.Code)
			}
		})
	}
}

func TestTokenIsRequiredForExactlyTheMutatingMethods(t *testing.T) {
	env := newAPITestEnv(t)

	// doRaw sends the request as-is, without the token do() would attach.
	for _, method := range mutatingMethods {
		t.Run("requires token for "+method, func(t *testing.T) {
			rec := env.doRaw(httptest.NewRequest(method, methodGateTestPath, nil))

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}

	for _, method := range nonMutatingMethods {
		t.Run("no token needed for "+method, func(t *testing.T) {
			rec := env.doRaw(httptest.NewRequest(method, "/api/mode", nil))

			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("status = %d, want the token gate not to fire", rec.Code)
			}
		})
	}
}
