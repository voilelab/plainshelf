package contract_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

func TestAPICoverContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Cover Me", "", "cover.txt", "body")
	url := apitest.BookURL(created.Meta.ID, "cover")

	rec := env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusNotFound)

	// The stored image format comes from the declared content type, so a body
	// that is not an image at all is refused.
	rec = env.PutContent(url, "text/plain", strings.NewReader("not image"))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)

	rec = env.PutContent(url, "image/png", bytes.NewReader(bytes.Repeat([]byte{'x'}, apitest.MaxBinaryUploadSize+1)))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	// Each accepted image type is stored and served back byte for byte under its
	// own content type, and the latest upload replaces the previous one.
	for _, tc := range []struct{ contentType, body string }{
		{"image/png", "fake png bytes"},
		{"image/webp", "fake webp bytes"},
	} {
		rec = env.PutContent(url, tc.contentType, strings.NewReader(tc.body))
		apitest.AssertStatus(t, rec, http.StatusNoContent)

		rec = env.Get(url)
		apitest.AssertStatus(t, rec, http.StatusOK)
		apitest.AssertContentType(t, rec, tc.contentType)
		if got := rec.Body.String(); got != tc.body {
			t.Fatalf("cover bytes = %q, want %q", got, tc.body)
		}
	}

	rec = env.Delete(url)
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}
