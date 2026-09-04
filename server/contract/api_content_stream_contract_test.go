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
	env := New(t)
	created := ImportTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	rec := env.PatchContent(SourceURL(bookID, sourceID, "content"), PlainTextContentType, strings.NewReader(""))
	AssertStatus(t, rec, http.StatusNoContent)

	AssertEmptyTextResponse(t, "book", env.GetWailsLike(BookURL(bookID, "content")))
	AssertEmptyTextResponse(t, "source", env.GetWailsLike(SourceURL(bookID, sourceID, "content")))

	// A log file is streamed by the same path, and an empty one is what a freshly
	// rotated log looks like.
	logDir := t.TempDir()
	logEnv := NewUnstarted(t, WithShelfLogDir(logDir, "shelf"))
	WriteLogFile(t, filepath.Join(logDir, "shelf-2024-01-02.log"), "")

	emptyLog := LogEntryByFilename(t, logEnv, "shelf-2024-01-02.log")
	AssertEmptyTextResponse(t, "log", logEnv.GetWailsLike(LogURL(emptyLog.ID, "content")))
}
