package contract_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestAPICoverContract(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Cover Me", "", "cover.txt", "body")
	url := BookURL(created.Meta.ID, "cover")

	rec := env.Get(url)
	AssertStatus(t, rec, http.StatusNotFound)

	// The stored image format comes from the declared content type, so a body
	// that is not an image at all is refused.
	rec = env.PutContent(url, "text/plain", strings.NewReader("not image"))
	AssertStatus(t, rec, http.StatusBadRequest)

	rec = env.PutContent(url, "image/png", bytes.NewReader(bytes.Repeat([]byte{'x'}, MaxBinaryUploadSize+1)))
	AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	// Each accepted image type is stored and served back byte for byte under its
	// own content type, and the latest upload replaces the previous one.
	for _, tc := range []struct{ contentType, body string }{
		{"image/png", "fake png bytes"},
		{"image/webp", "fake webp bytes"},
	} {
		rec = env.PutContent(url, tc.contentType, strings.NewReader(tc.body))
		AssertStatus(t, rec, http.StatusNoContent)

		rec = env.Get(url)
		AssertStatus(t, rec, http.StatusOK)
		AssertContentType(t, rec, tc.contentType)
		if got := rec.Body.String(); got != tc.body {
			t.Fatalf("cover bytes = %q, want %q", got, tc.body)
		}
	}

	rec = env.Delete(url)
	AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Get(url)
	AssertStatus(t, rec, http.StatusNotFound)
}
