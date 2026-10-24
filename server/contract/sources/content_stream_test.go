package sources_test

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

func TestAPIStreamContentReturns200ForEmptyFilesInWails(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	rec := env.PatchContent(apitest.SourceURL(bookID, sourceID, "content"), apitest.PlainTextContentType, strings.NewReader(""))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	apitest.AssertEmptyTextResponse(t, "book", env.GetWailsLike(apitest.BookURL(bookID, "content")))
	apitest.AssertEmptyTextResponse(t, "source", env.GetWailsLike(apitest.SourceURL(bookID, sourceID, "content")))

	logDir := t.TempDir()
	logEnv := apitest.NewUnstarted(t, apitest.WithShelfLogDir(logDir, "shelf"))
	apitest.WriteLogFile(t, filepath.Join(logDir, "shelf-2024-01-02.log"), "")

	emptyLog := apitest.LogEntryByFilename(t, logEnv, "shelf-2024-01-02.log")
	apitest.AssertEmptyTextResponse(t, "log", logEnv.GetWailsLike(apitest.LogURL(emptyLog.ID, "content")))
}

func TestAPIPatchContentRefusesOversizedBody(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Oversized Patch", "", "big.txt", "original")
	url := apitest.SourceURL(created.Meta.ID, created.Meta.CurrentSource, "content")
	before := env.Get(url).Body.String()

	body := io.LimitReader(repeatReader('x'), apitest.MaxTextUploadSize+1)
	rec := env.PatchContent(url, apitest.PlainTextContentType, body)
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	rec = env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != before {
		t.Fatalf("content after refused PATCH = %q, want the original", got)
	}
}

func TestAPIPatchContentOversizeUnsupportedEncoding(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Oversized UTF-32", "", "big.txt", "original")
	url := apitest.SourceURL(created.Meta.ID, created.Meta.CurrentSource, "content")

	utf32BOM := strings.NewReader("\xff\xfe\x00\x00")
	body := io.MultiReader(utf32BOM, io.LimitReader(repeatReader('x'), apitest.MaxTextUploadSize))
	rec := env.PatchContent(url, apitest.PlainTextContentType, body)
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)
}

type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}
