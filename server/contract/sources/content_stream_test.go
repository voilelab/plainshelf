package sources_test

import (
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
