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
var methodGateTestPath = ShelfURL("folders", "blocked")

func TestAPIReadOnlyModeRejectsExactlyTheMutatingMethodsContract(t *testing.T) {
	env := New(t)
	env.SetReadOnly(t, true)

	for _, method := range mutatingMethods {
		t.Run("rejects "+method, func(t *testing.T) {
			rec := env.Do(httptest.NewRequest(method, methodGateTestPath, nil))

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}

	for _, method := range nonMutatingMethods {
		t.Run("allows "+method, func(t *testing.T) {
			rec := env.Do(httptest.NewRequest(method, "/api/mode", nil))

			if rec.Code == http.StatusForbidden {
				t.Fatalf("status = %d, want the read-only gate not to fire", rec.Code)
			}
		})
	}
}

func TestAPITokenIsRequiredForExactlyTheMutatingMethodsContract(t *testing.T) {
	env := New(t)

	// DoRaw sends the request as-is, without the token Do() would attach.
	for _, method := range mutatingMethods {
		t.Run("requires token for "+method, func(t *testing.T) {
			rec := env.DoRaw(httptest.NewRequest(method, methodGateTestPath, nil))

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}

	for _, method := range nonMutatingMethods {
		t.Run("no token needed for "+method, func(t *testing.T) {
			rec := env.DoRaw(httptest.NewRequest(method, "/api/mode", nil))

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
	env := New(t, WithSecondShelf(t.TempDir()))
	book := ImportTextBook(t, env, "Gated", "fiction", "gated.txt", "body")
	bookID := book.Meta.ID
	assetPath := AssetURL(bookID, env.CurrentSourceID(t, bookID), "img-0001.png")

	cases := []struct {
		name   string
		method string
		url    string
		body   []byte
	}{
		{"book batch", http.MethodPost, BookBatchURL(),
			[]byte(`{"operation":"trash","book_ids":["book"]}`)},
		{"book cache export", http.MethodPost, BookCacheExportURL(), nil},
		{"book copy", http.MethodPost, BookCopiesURL(bookID), []byte("{}")},
		{"book transfer", http.MethodPost, BookTransfersURL(bookID),
			[]byte(`{"mode":"copy","target_shelf":"` + SecondShelfID + `"}`)},
		{"content stat refresh", http.MethodPost, ContentStatsURL(), nil},
		{"folder transfer", http.MethodPost, FolderTransfersURL(),
			[]byte(`{"mode":"copy","source_folder":["fiction"],"target_shelf":"` + SecondShelfID + `","target_folder":["imported"]}`)},
		{"source fingerprints", http.MethodPost, SourceFingerprintsURL(), nil},
		{"empty trash", http.MethodPost, EmptyTrashURL(), nil},
		{"asset write", http.MethodPut, assetPath, []byte("x")},
		{"asset delete", http.MethodDelete, assetPath, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			AssertMutationGated(t, env, tc.method, tc.url, tc.body)
		})
	}
}
