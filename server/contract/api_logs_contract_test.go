package contract_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIGetLogsContract(t *testing.T) {
	appLogDir := t.TempDir()
	appLogFile := filepath.Join(appLogDir, "app.log")
	shelfLogDir := t.TempDir()
	app, err := server.NewApp(&server.AppConf{
		Logger: logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:     logutil.LogFileTypeName,
				Filename: appLogFile,
			},
		},
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					Logger: logutil.LogConf{
						LogFile: logutil.LogFileConf{
							Type:   logutil.LogFileTypeNameRotate,
							Dir:    shelfLogDir,
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
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	handler := app.Handler()

	if err := os.WriteFile(appLogFile, []byte("app log"), 0o644); err != nil {
		t.Fatalf("WriteFile app log: %v", err)
	}
	appLogTime := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(appLogFile, appLogTime, appLogTime); err != nil {
		t.Fatalf("Chtimes app log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shelfLogDir, "shelf-2024-01-02.log"), []byte("shelf log"), 0o644); err != nil {
		t.Fatalf("WriteFile shelf log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shelfLogDir, "ignore.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("WriteFile ignore file: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	logs := decodeJSON[[]server.LogFileEntry](t, rec)
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want 2", len(logs))
	}

	if logs[0].ID == "" || logs[0].Source != "logger" || logs[0].Filename != "app.log" || logs[0].Date == "" {
		t.Fatalf("first log = %#v, want app logger entry", logs[0])
	}
	if logs[1].ID == "" || logs[1].Source != "shelves[0].shelfconf.logger" || logs[1].Filename != "shelf-2024-01-02.log" || logs[1].Date != "2024-01-02" {
		t.Fatalf("second log = %#v, want shelf logger entry", logs[1])
	}
}

func TestAPIGetLogContentContract(t *testing.T) {
	logDir := t.TempDir()
	app, err := server.NewApp(&server.AppConf{
		Logger: logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:   logutil.LogFileTypeNameRotate,
				Dir:    logDir,
				Prefix: "app",
			},
		},
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
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
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(logDir, "app-2024-01-02.log"), []byte("line 1\nline 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile log: %v", err)
	}

	handler := app.Handler()
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, listRec, http.StatusOK)

	logs := decodeJSON[[]server.LogFileEntry](t, listRec)
	var target *server.LogFileEntry
	for i := range logs {
		if logs[i].Filename == "app-2024-01-02.log" {
			target = &logs[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("log list missing seeded file: %#v", logs)
	}

	contentRec := httptest.NewRecorder()
	handler.ServeHTTP(contentRec, httptest.NewRequest(http.MethodGet, "/api/logs/"+target.ID+"/content", nil))
	assertStatus(t, contentRec, http.StatusOK)
	if got := contentRec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := contentRec.Body.String(); got != "line 1\nline 2\n" {
		t.Fatalf("content = %q, want seeded log content", got)
	}

	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/api/logs/missing/content", nil))
	assertStatus(t, missingRec, http.StatusNotFound)
}
