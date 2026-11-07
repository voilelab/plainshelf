package sources_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPISourceSizeContract(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Size", "", "size.txt", "text")
	images := "missing.png\npresent.jpg\n"
	sourceID := createImagesSource(t, env, book.Meta.ID, images)
	env.WriteSourceAsset(t, book.Meta.ID, sourceID, "present.jpg", []byte("image"))
	env.WriteSourceAsset(t, book.Meta.ID, sourceID, "unlisted.png", []byte("stray"))
	for _, tc := range []struct {
		id   string
		size int
	}{
		{sourceID, len(images) + 10},
		{book.Meta.CurrentSource, len("text\n世界")},
	} {
		meta := apitest.GetJSON[map[string]any](t, env, apitest.SourceURL(book.Meta.ID, tc.id))
		if meta["size_bytes"] != float64(tc.size) {
			t.Fatalf("source size = %v, want %d", meta["size_bytes"], tc.size)
		}
	}
	list := env.Get(apitest.BookURL(book.Meta.ID, "sources"))
	apitest.AssertStatus(t, list, http.StatusOK)
	if strings.Contains(list.Body.String(), "size_bytes") {
		t.Fatal("source list includes size")
	}
	books := env.Get(apitest.ShelfURL("books"))
	apitest.AssertStatus(t, books, http.StatusOK)
	if strings.Contains(books.Body.String(), "size_bytes") {
		t.Fatal("book list includes size")
	}

	apitest.AssertStatus(t, env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)
	if raw := testutil.WaitForExportedBookCache(t, env.LibRoot); strings.Contains(string(raw), "size_bytes") {
		t.Fatal("exported book cache includes source size")
	}
	sourceDir := filepath.Join(filepath.Dir(env.BookMetaPath(t, book.Meta.ID)), shelf.SourcesFolder, sourceID)
	if err := os.Remove(filepath.Join(sourceDir, "source.txt")); err != nil {
		t.Fatal(err)
	}
	meta := apitest.GetJSON[map[string]any](t, env, apitest.SourceURL(book.Meta.ID, sourceID))
	if _, ok := meta["size_bytes"]; ok {
		t.Fatalf("unavailable size = %v, want omitted", meta["size_bytes"])
	}
}

func TestAPIVirtualSourceSize(t *testing.T) {
	root, ids := apitest.SeedVirtualSource(t, "Remote")
	env := apitest.New(t)
	env.AddVirtualShelf(t, root)
	bookID := ids["Remote"]
	url := apitest.ShelfIDURL(apitest.VirtualShelfID, "books", bookID, "sources")
	sources := apitest.GetJSON[[]shelf.SourceMeta](t, env, url)
	url = apitest.ShelfIDURL(apitest.VirtualShelfID, "books", bookID, "sources", sources[0].ID)
	meta := apitest.GetJSON[map[string]any](t, env, url)
	if meta["size_bytes"] != float64(len("Remote text")) {
		t.Fatalf("read-only size = %v", meta["size_bytes"])
	}
}
