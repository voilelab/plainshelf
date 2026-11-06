package books_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIImportCBZ(t *testing.T) {
	archive := testutil.BuildCBZ(t, map[string][]byte{
		"2.png":         testutil.OnePixelPNG(),
		"ComicInfo.xml": []byte(`<ComicInfo><Title>Comic</Title><Writer>Author</Writer><Manga>YesAndRightToLeft</Manga></ComicInfo>`),
	})
	for _, media := range []string{"application/vnd.comicbook+zip", "application/x-cbz", "application/zip", "application/octet-stream", ""} {
		t.Run(media, func(t *testing.T) {
			env := apitest.New(t)
			book := apitest.ImportFileBook(t, env, "book.CBZ", media, string(archive))
			if book.Meta == nil || book.Meta.Format != "images" || book.Meta.Title != "Comic" || book.Meta.CurrentSource == "" {
				t.Fatalf("book = %+v", book)
			}
			source := apitest.GetJSON[shelf.SourceMeta](t, env, apitest.SourceURL(book.Meta.ID, book.Meta.CurrentSource))
			if source.Format != "images" || source.PageDirection != "rtl" {
				t.Fatalf("source = %+v", source)
			}
			rec := env.Get(apitest.SourceURL(book.Meta.ID, source.ID, "pages"))
			apitest.AssertStatus(t, rec, http.StatusOK)
		})
	}
	env := apitest.New(t)
	rec := apitest.PostBookImport(t, env, apitest.BookUpload("book.cbz", "application/zip", string(archive), [2]string{"title", "My title"}, [2]string{"folder", "Comics"}))
	apitest.AssertStatus(t, rec, http.StatusCreated)
	book := apitest.DecodeJSON[server.Book](t, rec)
	if book.Meta.Title != "My title" || strings.Join(book.Folder, "/") != "Comics" {
		t.Fatalf("book = %+v", book)
	}
}

func TestAPIImportCBZFailure(t *testing.T) {
	env := apitest.New(t)
	for _, tc := range []struct{ body, want string }{
		{"not a zip", "not a readable cbz"},
		{string(testutil.BuildCBZ(t, map[string][]byte{"../1.png": testutil.OnePixelPNG()})), "path traversal"},
		{string(testutil.BuildCBZ(t, map[string][]byte{"1.jpg": []byte("broken")})), "invalid cbz image"},
	} {
		rec := apitest.PostBookImport(t, env, apitest.BookUpload("book.cbz", "application/zip", tc.body))
		apitest.AssertStatus(t, rec, http.StatusBadRequest)
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}
	if ids := apitest.ListedBookIDs(t, env); len(ids) != 0 {
		t.Fatalf("failed imports left books: %v", ids)
	}
}
