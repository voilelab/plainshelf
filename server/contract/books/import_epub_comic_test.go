package books_test

import (
	"bytes"
	"image"
	_ "image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

const imagesStrategy = `{"preset":"images","include_description":true}`

func TestAPIImportEPUBAsImages(t *testing.T) {
	env := apitest.New(t)
	imported := apitest.ImportEPUBWithStrategy(t, env, "comic.epub",
		string(testutil.BuildComicTestEPUB(t, 3)), imagesStrategy)

	if imported.Meta.Format != shelf.BookFormatImages || imported.Meta.Title != testutil.ComicEPUBTitle ||
		strings.Join(imported.Meta.Authors, ",") != "Comic Author" || imported.Meta.Language != "zh-Hant" {
		t.Fatalf("meta = %+v", imported.Meta)
	}
	bookID, sourceID := imported.Meta.ID, imported.Meta.CurrentSource

	rec := env.Get(apitest.SourceURL(bookID, sourceID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	pages := strings.Fields(rec.Body.String())
	if len(pages) != 3 {
		t.Fatalf("page list = %q, want the 3 pages without the cover pages", rec.Body.String())
	}

	for i, name := range pages {
		rec = env.Get(apitest.AssetURL(bookID, sourceID, name))
		apitest.AssertStatus(t, rec, http.StatusOK)
		config, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
		if err != nil || config.Width != i+1 {
			t.Fatalf("page %d %q width = %d (%v), want %d", i, name, config.Width, err, i+1)
		}
	}
}

func TestAPIImportEPUBAsImagesRejectsText(t *testing.T) {
	env := apitest.New(t)
	rec := apitest.PostBookImport(t, env, apitest.BookUpload("novel.epub", "application/epub+zip",
		string(testutil.BuildTestEPUB(t)), [2]string{"strategy", imagesStrategy}))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "contains text") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if ids := apitest.ListedBookIDs(t, env); len(ids) != 0 {
		t.Fatalf("failed import left books: %v", ids)
	}
}
