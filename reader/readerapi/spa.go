package readerapi

import (
	"bytes"
	"encoding/json/v2"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

type BootConfig struct {
	ShelfID string `json:"shelf_id"`
	BookID  string `json:"book_id"`

	Section *int `json:"section,omitempty"`
}

const bootMarker = "window.__PLAINSHELF_READER__"

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
		w.Write(page)
	})
}

func indexHTML(assets fs.FS, boot BootConfig) ([]byte, error) {
	page, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	encoded, err := json.Marshal(boot, jsonopt.API())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	// encoding/json/v2 leaves "<" unescaped; HTML script parsing requires
	// escaping it to prevent data from closing the script element.
	encoded = bytes.ReplaceAll(encoded, []byte("<"), []byte(`\u003c`))

	script := []byte("<script>" + bootMarker + " = " + string(encoded) + ";</script>")

	if index := bytes.Index(page, []byte("</head>")); index >= 0 {
		return slices.Concat(page[:index], script, page[index:]), nil
	}
	return slices.Concat(script, page), nil
}
