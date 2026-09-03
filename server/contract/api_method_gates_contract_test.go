package contract_test

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
var methodGateTestPath = shelfURL("folders", "blocked")

func TestAPIReadOnlyModeRejectsExactlyTheMutatingMethodsContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.setReadOnly(t, true)

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

func TestAPITokenIsRequiredForExactlyTheMutatingMethodsContract(t *testing.T) {
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

// Every mutating route sits behind the same two gates, so they are enumerated
// once here instead of once per endpoint file. A route missing from this table
// is a route nobody proved is gated, which is easier to notice in one list than
// in the absence of a test somewhere else.
func TestAPIMutatingRoutesAreGatedContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	book := importTextBook(t, env, "Gated", "fiction", "gated.txt", "body")
	bookID := book.Meta.ID
	assetPath := assetURL(bookID, env.currentSourceID(t, bookID), "img-0001.png")

	cases := []struct {
		name   string
		method string
		url    string
		body   []byte
	}{
		{"book batch", http.MethodPost, bookBatchURL(),
			[]byte(`{"operation":"trash","book_ids":["book"]}`)},
		{"book cache export", http.MethodPost, bookCacheExportURL(), nil},
		{"book copy", http.MethodPost, bookCopiesURL(bookID), []byte("{}")},
		{"book transfer", http.MethodPost, bookTransfersURL(bookID),
			[]byte(`{"mode":"copy","target_shelf":"` + secondShelfID + `"}`)},
		{"content stat refresh", http.MethodPost, contentStatsURL(), nil},
		{"folder transfer", http.MethodPost, folderTransfersURL(),
			[]byte(`{"mode":"copy","source_folder":["fiction"],"target_shelf":"` + secondShelfID + `","target_folder":["imported"]}`)},
		{"source fingerprints", http.MethodPost, sourceFingerprintsURL(), nil},
		{"empty trash", http.MethodPost, emptyTrashURL(), nil},
		{"asset write", http.MethodPut, assetPath, []byte("x")},
		{"asset delete", http.MethodDelete, assetPath, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertMutationGated(t, env, tc.method, tc.url, tc.body)
		})
	}
}
