package readerapi

import (
	"bytes"
	"encoding/json/v2"
	"io/fs"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

// BootConfig is what the window tells the frontend before it asks anything.
//
// The reader shell needs the open book's ID to route straight to it, and asking
// for it over HTTP would mean a first paint with no book — so it is injected
// into index.html instead. BookID is empty while no book is open, which is what
// the shell renders its holding screen for.
type BootConfig struct {
	ShelfID string `json:"shelf_id"`
	BookID  string `json:"book_id"`
	// Section is the reader's own section index to open at, matching the
	// frontend's `?section=` deep link. It is set only when the reader was
	// launched for a specific chapter (desktop -section); nil — and omitted from
	// the injected JSON — means "open at the restored progress", the standalone
	// default. A pointer so section 0 (the first chapter) is still carried.
	Section *int `json:"section,omitempty"`
}

// bootScript is the injected global. The marker is what the frontend's
// isReaderRuntime() looks for, so it is defined even when no book is open.
const bootMarker = "window.__PLAINSHELF_READER__"

// NewSPAHandler serves the embedded frontend with the reader's boot config
// injected, falling back to index.html for in-app routes.
//
// The fallback is explicit rather than left to the asset server: the window
// reloads itself at /reader/{id} when a book is opened, and that path is a Vue
// route, not a file.
func NewSPAHandler(assets fs.FS, boot func() BootConfig) http.Handler {
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && name != "index.html" {
			if _, err := fs.Stat(assets, name); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}

		page, err := indexHTML(assets, boot())
		if err != nil {
			http.Error(w, "failed to render the app shell", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page) //nolint:errcheck // the status is already written
	})
}

// indexHTML returns the embedded index.html with the boot config injected
// ahead of the app's own scripts.
func indexHTML(assets fs.FS, boot BootConfig) ([]byte, error) {
	page, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	encoded, err := json.Marshal(boot, jsonopt.API())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	// A book ID cannot close the script element: "<" is the only character that
	// could, and a \u003c escape survives JSON.parse unchanged. v2 does not
	// escape "<" itself, so this replacement is the only thing standing there.
	encoded = bytes.ReplaceAll(encoded, []byte("<"), []byte(`\u003c`))

	script := []byte("<script>" + bootMarker + " = " + string(encoded) + ";</script>")

	// Before </head>, so the flag is set before main.js runs and bootstrap can
	// decide which shell to install without waiting for a request.
	if index := bytes.Index(page, []byte("</head>")); index >= 0 {
		return bytes.Join([][]byte{page[:index], script, page[index:]}, nil), nil
	}
	return bytes.Join([][]byte{script, page}, nil), nil
}
