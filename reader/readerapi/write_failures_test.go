package readerapi_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header       { return w.header }
func (w *failingWriter) WriteHeader(int)           {}
func (w *failingWriter) Write([]byte) (int, error) { return 0, errors.New("client went away") }

func TestHandlerLogsShortStream(t *testing.T) {
	var logs bytes.Buffer
	library := readerapi.NewLibrary(&logutil.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	t.Cleanup(func() { library.Close() })
	if _, err := library.Open(writeBookPackage(t, "Dune", testBookID)); err != nil {
		t.Fatalf("Open: %v", err)
	}

	handler := readerapi.NewHandler(library, "test")
	handler.ServeHTTP(&failingWriter{header: http.Header{}}, httptest.NewRequest(http.MethodGet, bookPath(testBookID)+"/content", nil))

	line := logs.String()
	for _, want := range []string{"source stream ended early", "book_id=" + testBookID, "source_id=" + testSourceID, "client went away"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "beginning") {
		t.Errorf("log %q carries the book's text", line)
	}
}
