package books_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIBookCopyInPlaceContract(t *testing.T) {
	for name, body := range map[string]string{"empty object": "{}", "no body": ""} {
		t.Run(name, func(t *testing.T) {
			env := apitest.New(t)
			original := apitest.ImportTextBook(t, env, "Copy Me", "fiction", "copyme.txt", "body")

			var reader io.Reader
			if body != "" {
				reader = strings.NewReader(body)
			}
			rec := env.Post(apitest.BookCopiesURL(original.Meta.ID), reader)
			apitest.AssertStatus(t, rec, http.StatusCreated)
			apitest.AssertJSONContentType(t, rec)
			copied := apitest.DecodeJSON[server.Book](t, rec)

			if copied.Meta.ID == "" || copied.Meta.ID == original.Meta.ID {
				t.Fatalf("copy id = %q, want a fresh id distinct from %q", copied.Meta.ID, original.Meta.ID)
			}
			if !copied.Folder.Equal(original.Folder) {
				t.Errorf("copy folder = %v, want the source folder %v", copied.Folder, original.Folder)
			}
			if copied.Meta.Title != "Copy Me" {
				t.Errorf("copy title = %q, want %q", copied.Meta.Title, "Copy Me")
			}

			books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL())
			if len(books) != 2 {
				t.Fatalf("books listed = %d, want the original and its copy", len(books))
			}
		})
	}
}

func TestAPIBookCopyToFolderContract(t *testing.T) {
	env := apitest.New(t)
	original := apitest.ImportTextBook(t, env, "Relocate", "origin", "relocate.txt", "body")

	rec := env.Post(apitest.BookCopiesURL(original.Meta.ID), strings.NewReader(`{"folder":["archive","2026"]}`))
	apitest.AssertStatus(t, rec, http.StatusCreated)
	copied := apitest.DecodeJSON[server.Book](t, rec)

	if !copied.Folder.Equal(shelf.FolderPath{"archive", "2026"}) {
		t.Errorf("copy folder = %v, want [archive 2026]", copied.Folder)
	}
	if copied.Meta.ID == original.Meta.ID {
		t.Errorf("copy id = %q, want it distinct from the original", copied.Meta.ID)
	}
}

func TestAPIBookCopyMissingContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.Post(apitest.BookCopiesURL("missing-book"), strings.NewReader("{}"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPIBookCopyInvalidFolderContract(t *testing.T) {
	env := apitest.New(t)
	original := apitest.ImportTextBook(t, env, "Bad Target", "", "bad.txt", "body")

	rec := env.Post(apitest.BookCopiesURL(original.Meta.ID), strings.NewReader(`{"folder":[".."]}`))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
}
