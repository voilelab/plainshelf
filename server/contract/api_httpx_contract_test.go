package contract_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAPIUnknownShelfIsRejectedConsistentlyContract(t *testing.T) {
	env := newAPITestEnv(t)

	paths := []string{
		shelfIDURL("missing_shelf", "books"),
		shelfIDURL("missing_shelf", "status"),
		shelfIDURL("missing_shelf", "layers"),
		shelfIDURL("missing_shelf", "trash", "books"),
		shelfIDURL("missing_shelf", "books", "duplicate"),
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := env.get(path)

			assertStatus(t, rec, http.StatusNotFound)
			if got := strings.TrimSpace(rec.Body.String()); got != "shelf not found" {
				t.Fatalf("body = %q, want %q", got, "shelf not found")
			}
		})
	}
}

func TestAPIJSONResponsesShareOneContentTypeContract(t *testing.T) {
	env := newAPITestEnv(t)

	paths := []string{
		"/api/mode",
		"/api/version",
		"/api/shelves",
		booksURL(),
		"/api/setting/epub_import_strategy",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := env.get(path)

			assertStatus(t, rec, http.StatusOK)
			assertJSONContentType(t, rec)
		})
	}
}

func TestAPIJSONBodiesEndWithANewlineContract(t *testing.T) {
	env := newAPITestEnv(t)

	// A client reading the body line-wise would block without the newline.
	rec := env.get("/api/version")

	assertStatus(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.HasSuffix(body, "\n") {
		t.Fatalf("body = %q, want a trailing newline", body)
	}
}
