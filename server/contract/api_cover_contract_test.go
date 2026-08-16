package contract_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestAPICoverContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Cover Me", "", "cover.txt", "body")
	url := bookURL(created.Meta.ID, "cover")

	rec := env.get(url)
	assertStatus(t, rec, http.StatusNotFound)

	// The stored image format comes from the declared content type, so a body
	// that is not an image at all is refused.
	rec = env.putContent(url, "text/plain", strings.NewReader("not image"))
	assertStatus(t, rec, http.StatusBadRequest)

	rec = env.putContent(url, "image/png", bytes.NewReader(bytes.Repeat([]byte{'x'}, maxBinaryUploadSize+1)))
	assertStatus(t, rec, http.StatusRequestEntityTooLarge)

	// Each accepted image type is stored and served back byte for byte under its
	// own content type, and the latest upload replaces the previous one.
	for _, tc := range []struct{ contentType, body string }{
		{"image/png", "fake png bytes"},
		{"image/webp", "fake webp bytes"},
	} {
		rec = env.putContent(url, tc.contentType, strings.NewReader(tc.body))
		assertStatus(t, rec, http.StatusNoContent)

		rec = env.get(url)
		assertStatus(t, rec, http.StatusOK)
		assertContentType(t, rec, tc.contentType)
		if got := rec.Body.String(); got != tc.body {
			t.Fatalf("cover bytes = %q, want %q", got, tc.body)
		}
	}

	rec = env.delete(url)
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.get(url)
	assertStatus(t, rec, http.StatusNotFound)
}
