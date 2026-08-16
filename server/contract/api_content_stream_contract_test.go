package contract_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIStreamContentReturns200ForEmptyFilesInWails(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+bookID+"/sources/"+sourceID+"/content", strings.NewReader(""))
	updateReq.Header.Set("Content-Type", "text/plain; charset=utf-8")
	updateRec := env.do(updateReq)
	assertStatus(t, updateRec, http.StatusNoContent)

	bookContentRec := newWailsLikeRecorder()
	env.handler.ServeHTTP(bookContentRec, httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+bookID+"/content", nil))
	assertWailsLikeStatus(t, bookContentRec, http.StatusOK)
	if got := bookContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("book Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := bookContentRec.body.String(); got != "" {
		t.Fatalf("book content = %q, want empty", got)
	}

	sourceContentRec := newWailsLikeRecorder()
	env.handler.ServeHTTP(sourceContentRec, httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+bookID+"/sources/"+sourceID+"/content", nil))
	assertWailsLikeStatus(t, sourceContentRec, http.StatusOK)
	if got := sourceContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("source Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := sourceContentRec.body.String(); got != "" {
		t.Fatalf("source content = %q, want empty", got)
	}

	logDir := t.TempDir()
	logApp, err := server.NewApp(&server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					Logger: logutil.LogConf{
						LogFile: logutil.LogFileConf{
							Type:   logutil.LogFileTypeNameRotate,
							Dir:    logDir,
							Prefix: "shelf",
						},
					},
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := logApp.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(logDir, "shelf-2024-01-02.log"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile empty log: %v", err)
	}

	listRec := httptest.NewRecorder()
	logHandler := logApp.Handler()
	logHandler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, listRec, http.StatusOK)

	logs := decodeJSON[[]server.LogFileEntry](t, listRec)
	var emptyLogID string
	for i := range logs {
		if logs[i].Filename == "shelf-2024-01-02.log" {
			emptyLogID = logs[i].ID
			break
		}
	}
	if emptyLogID == "" {
		t.Fatalf("expected empty shelf log in list, got %#v", logs)
	}

	logContentRec := newWailsLikeRecorder()
	logHandler.ServeHTTP(logContentRec, httptest.NewRequest(http.MethodGet, "/api/logs/"+emptyLogID+"/content", nil))
	assertWailsLikeStatus(t, logContentRec, http.StatusOK)
	if got := logContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("log Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := logContentRec.body.String(); got != "" {
		t.Fatalf("log content = %q, want empty", got)
	}
}
