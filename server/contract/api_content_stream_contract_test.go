package contract_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// An empty file is a legitimate state: a source can be cleared, and a log can be
// rotated before anything is written to it. The Wails asset server reports
// whatever status the handler wrote, so a handler that streams zero bytes without
// writing a header would surface there as a failure rather than an empty body.
func TestAPIStreamContentReturns200ForEmptyFilesInWails(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	rec := env.patchContent(sourceURL(bookID, sourceID, "content"), plainTextContentType, strings.NewReader(""))
	assertStatus(t, rec, http.StatusNoContent)

	assertEmptyTextResponse(t, "book", env.getWailsLike(bookURL(bookID, "content")))
	assertEmptyTextResponse(t, "source", env.getWailsLike(sourceURL(bookID, sourceID, "content")))

	// A log file is streamed by the same path, and an empty one is what a freshly
	// rotated log looks like.
	logDir := t.TempDir()
	logEnv := newUnstartedAPITestEnv(t, withShelfLogDir(logDir, "shelf"))
	writeLogFile(t, filepath.Join(logDir, "shelf-2024-01-02.log"), "")

	emptyLog := logEntryByFilename(t, logEnv, "shelf-2024-01-02.log")
	assertEmptyTextResponse(t, "log", logEnv.getWailsLike(logURL(emptyLog.ID, "content")))
}
