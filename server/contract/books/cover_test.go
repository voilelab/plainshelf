package books_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
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

	rec = env.PutContent(url, "text/plain", strings.NewReader("not image"))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)

	rec = env.PutContent(url, "image/png", bytes.NewReader(bytes.Repeat([]byte{'x'}, apitest.MaxBinaryUploadSize+1)))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

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

func pngHeader(width, height uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8] = 8
	ihdr[9] = 6

	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr...)
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func TestAPICoverJPGPixelLimit(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Huge Cover", "", "huge.txt", "body")
	url := apitest.BookURL(created.Meta.ID, "cover")

	rec := env.Post(apitest.SettingURL("cover_to_jpg"), strings.NewReader("true"))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	rec = env.PutContent(url, "image/png", bytes.NewReader(pngHeader(20_000, 20_000)))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	rec = env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}
