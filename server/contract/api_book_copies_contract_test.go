package contract_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func bookCopiesURL(bookID string) string {
	return bookURL(bookID, "copies")
}

// POST .../books/{id}/copies answers with the copy: a fresh id and the folder it
// landed in. With no destination in the body the copy stays in the source book's
// own folder, and both books are then listed - the outcome a hand-made cp -r
// cannot reach, because two books sharing an id collapse to one. Every field of
// the request is optional, so no body at all means the same thing as an empty
// object.
func TestAPIBookCopyInPlaceContract(t *testing.T) {
	for name, body := range map[string]string{"empty object": "{}", "no body": ""} {
		t.Run(name, func(t *testing.T) {
			env := newAPITestEnv(t)
			original := importTextBook(t, env, "Copy Me", "fiction", "copyme.txt", "body")

			var reader io.Reader
			if body != "" {
				reader = strings.NewReader(body)
			}
			rec := env.post(bookCopiesURL(original.Meta.ID), reader)
			assertStatus(t, rec, http.StatusCreated)
			assertJSONContentType(t, rec)
			copied := decodeJSON[server.Book](t, rec)

			if copied.Meta.ID == "" || copied.Meta.ID == original.Meta.ID {
				t.Fatalf("copy id = %q, want a fresh id distinct from %q", copied.Meta.ID, original.Meta.ID)
			}
			if !copied.Folder.Equal(original.Folder) {
				t.Errorf("copy folder = %v, want the source folder %v", copied.Folder, original.Folder)
			}
			if copied.Meta.Title != "Copy Me" {
				t.Errorf("copy title = %q, want %q", copied.Meta.Title, "Copy Me")
			}

			books := getJSON[[]server.Book](t, env, booksURL())
			if len(books) != 2 {
				t.Fatalf("books listed = %d, want the original and its copy", len(books))
			}
		})
	}
}

// A copy can name a destination folder in the body and lands there.
func TestAPIBookCopyToFolderContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "Relocate", "origin", "relocate.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), strings.NewReader(`{"folder":["archive","2026"]}`))
	assertStatus(t, rec, http.StatusCreated)
	copied := decodeJSON[server.Book](t, rec)

	if !copied.Folder.Equal(shelf.FolderPath{"archive", "2026"}) {
		t.Errorf("copy folder = %v, want [archive 2026]", copied.Folder)
	}
	if copied.Meta.ID == original.Meta.ID {
		t.Errorf("copy id = %q, want it distinct from the original", copied.Meta.ID)
	}
}

// Copying an unknown book is a 404, even with an otherwise valid body.
func TestAPIBookCopyMissingContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.post(bookCopiesURL("missing-book"), strings.NewReader("{}"))
	assertStatus(t, rec, http.StatusNotFound)
}

// An invalid destination folder is rejected with 400 before anything is copied.
func TestAPIBookCopyInvalidFolderContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "Bad Target", "", "bad.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), strings.NewReader(`{"folder":[".."]}`))
	assertStatus(t, rec, http.StatusBadRequest)
}
