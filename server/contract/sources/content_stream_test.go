package sources_test

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

// An empty file is a legitimate state: a source can be cleared, and a log can be
// rotated before anything is written to it. The Wails asset server reports
// whatever status the handler wrote, so a handler that streams zero bytes without
// writing a header would surface there as a failure rather than an empty body.
func TestAPIStreamContentReturns200ForEmptyFilesInWails(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	rec := env.PatchContent(apitest.SourceURL(bookID, sourceID, "content"), apitest.PlainTextContentType, strings.NewReader(""))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	apitest.AssertEmptyTextResponse(t, "book", env.GetWailsLike(apitest.BookURL(bookID, "content")))
	apitest.AssertEmptyTextResponse(t, "source", env.GetWailsLike(apitest.SourceURL(bookID, sourceID, "content")))

	// A log file is streamed by the same path, and an empty one is what a freshly
	// rotated log looks like.
	logDir := t.TempDir()
	logEnv := apitest.NewUnstarted(t, apitest.WithShelfLogDir(logDir, "shelf"))
	apitest.WriteLogFile(t, filepath.Join(logDir, "shelf-2024-01-02.log"), "")

	emptyLog := apitest.LogEntryByFilename(t, logEnv, "shelf-2024-01-02.log")
	apitest.AssertEmptyTextResponse(t, "log", logEnv.GetWailsLike(apitest.LogURL(emptyLog.ID, "content")))
}

// The body streams into the write rather than being read first, so the limit
// trips mid-write; it must still answer 413 and leave the old content in place.
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

// repeatReader yields its byte forever without holding the body in memory.
type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}
