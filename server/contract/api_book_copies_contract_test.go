package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func bookCopiesURL(bookID string) string {
	return bookURL(bookID, "copies")
}

// POST .../books/{id}/copies answers with the copy: a fresh id and the layer it
// landed in. With no destination in the body the copy stays in the source book's
// own layer, and both books are then listed - the outcome a hand-made cp -r
// cannot reach, because two books sharing an id collapse to one.
func TestAPIBookCopyInPlaceContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "Copy Me", "fiction", "copyme.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), strings.NewReader("{}"))
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	copied := decodeJSON[server.Book](t, rec)

	if copied.Meta.ID == "" || copied.Meta.ID == original.Meta.ID {
		t.Fatalf("copy id = %q, want a fresh id distinct from %q", copied.Meta.ID, original.Meta.ID)
	}
	if !copied.Layer.Equal(original.Layer) {
		t.Errorf("copy layer = %v, want the source layer %v", copied.Layer, original.Layer)
	}
	if copied.Meta.Title != "Copy Me" {
		t.Errorf("copy title = %q, want %q", copied.Meta.Title, "Copy Me")
	}

	books := getJSON[[]server.Book](t, env, booksURL())
	if len(books) != 2 {
		t.Fatalf("books listed = %d, want the original and its copy", len(books))
	}
}

// An empty request body is a valid "copy in place": every field of the copy
// request is optional, so no body means the source book's own layer.
func TestAPIBookCopyEmptyBodyContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "No Body", "shelf", "nobody.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), nil)
	assertStatus(t, rec, http.StatusCreated)
	copied := decodeJSON[server.Book](t, rec)

	if copied.Meta.ID == original.Meta.ID {
		t.Errorf("copy id = %q, want it distinct from the original", copied.Meta.ID)
	}
	if !copied.Layer.Equal(original.Layer) {
		t.Errorf("copy layer = %v, want the source layer %v", copied.Layer, original.Layer)
	}
}

// A copy can name a destination layer in the body and lands there.
func TestAPIBookCopyToLayerContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "Relocate", "origin", "relocate.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), strings.NewReader(`{"layer":["archive","2026"]}`))
	assertStatus(t, rec, http.StatusCreated)
	copied := decodeJSON[server.Book](t, rec)

	if !copied.Layer.Equal(shelf.FolderPath{"archive", "2026"}) {
		t.Errorf("copy layer = %v, want [archive 2026]", copied.Layer)
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

// An invalid destination layer is rejected with 400 before anything is copied.
func TestAPIBookCopyInvalidLayerContract(t *testing.T) {
	env := newAPITestEnv(t)
	original := importTextBook(t, env, "Bad Target", "", "bad.txt", "body")

	rec := env.post(bookCopiesURL(original.Meta.ID), strings.NewReader(`{"layer":[".."]}`))
	assertStatus(t, rec, http.StatusBadRequest)
}

// The copy endpoint stays behind both write gates: the token requirement and the
// read-only refusal.
func TestAPIBookCopyIsGatedContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Gated", "", "gated.txt", "body")

	assertMutationGated(t, env, http.MethodPost, bookCopiesURL(book.Meta.ID), []byte("{}"))
}
