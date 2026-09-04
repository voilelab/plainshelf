package books_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/shelf"
)

// TestAPIImportEPUBRecordsDroppedImagesContract pins the whole path: the
// converter counts the illustrations, the importer writes the note onto the
// source it created, and the sources endpoint hands it back for the book detail
// view to show.
func TestAPIImportEPUBRecordsDroppedImagesContract(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
		want    string
	}{
		{
			// Both plates are stored beside the text now, so an illustrated
			// EPUB loses nothing and leaves no note.
			archive: testutil.BuildIllustratedTestEPUB(t),
			want:    "",
		},
		{
			// A format the shelf cannot serve is still a loss, and is still
			// what the note is for.
			name:    "unstorable illustration is still reported",
			archive: testutil.BuildUnstorableImageTestEPUB(t),
			want:    "Converted from EPUB. 1 embedded image was dropped.",
		},
		{
			// The cover is kept, so an EPUB whose only image is the cover has
			// nothing to report.
			name:    "epub without illustrations records nothing",
			archive: testutil.BuildTestEPUB(t),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := apitest.New(t)
			imported := apitest.ImportFileBook(t, env, "book.epub", "application/epub+zip", string(tt.archive))

			if imported.Meta == nil || imported.Meta.CurrentSource == "" {
				t.Fatal("imported book has no current source")
			}

			source := apitest.GetJSON[shelf.SourceMeta](t, env,
				apitest.SourceURL(imported.Meta.ID, imported.Meta.CurrentSource))
			if source.Comment != tt.want {
				t.Errorf("source comment = %q, want %q", source.Comment, tt.want)
			}
		})
	}
}

// The illustrations an EPUB carried must land on the shelf, be referenced by
// the text, and come back through the asset route. This is the first test that
// exercises storage, conversion and serving together; each side alone can pass
// while the names they agree on have drifted apart.
func TestAPIImportEPUBStoresIllustrationsAsAssetsContract(t *testing.T) {
	env := apitest.New(t)
	imported := apitest.ImportFileBook(t, env, "book.epub", "application/epub+zip",
		string(testutil.BuildIllustratedTestEPUB(t)))

	if imported.Meta == nil || imported.Meta.CurrentSource == "" {
		t.Fatal("imported book has no current source")
	}
	bookID, sourceID := imported.Meta.ID, imported.Meta.CurrentSource

	rec := env.Get(apitest.SourceURL(bookID, sourceID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	content := rec.Body.String()

	// Both plates are stored, numbered in the order the spine reached them.
	for _, name := range []string{"img-0001.png", "img-0002.png"} {
		link := "![](" + shelf.SourceAssetsFolder + "/" + name + ")"
		if !strings.Contains(content, link) {
			t.Fatalf("converted text does not contain %q:\n%s", link, content)
		}

		rec = env.Get(apitest.AssetURL(bookID, sourceID, name))
		apitest.AssertStatus(t, rec, http.StatusOK)
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("asset %s Content-Type = %q, want image/png", name, got)
		}
		if !bytes.Equal(rec.Body.Bytes(), testutil.OnePixelPNG()) {
			t.Errorf("asset %s bytes do not match the archive entry", name)
		}
	}

	// The book is Markdown, which is the only format whose reader renders the
	// link that was just written into the text.
	if imported.Meta.Format != "md" {
		t.Errorf("imported format = %q, want md", imported.Meta.Format)
	}
}

// With the keep_images setting off, an import that submits a strategy of its
// own must still store no illustrations. The unit-level half of this is
// TestImportEPUBPerRequestStrategyInheritsKeepImages in package server.
func TestAPIImportEPUBHonoursKeepImagesOffContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.Post("/api/setting/epub_import_strategy",
		strings.NewReader(`{"preset":"markdown","include_description":true,"keep_images":false}`))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	imported := apitest.ImportEPUBWithStrategy(t, env, "book.epub",
		string(testutil.BuildIllustratedTestEPUB(t)),
		`{"preset":"markdown","include_description":true}`)

	bookID, sourceID := imported.Meta.ID, imported.Meta.CurrentSource

	rec = env.Get(apitest.SourceURL(bookID, sourceID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "![") {
		t.Fatalf("images were stored despite keep_images=false:\n%s", rec.Body.String())
	}

	apitest.AssertStatus(t, env.Get(apitest.AssetURL(bookID, sourceID, "img-0001.png")), http.StatusNotFound)
}
