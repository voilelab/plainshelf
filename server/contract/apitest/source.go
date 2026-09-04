package apitest

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// CreateBookSource creates an empty source on a book and returns its ID, which is
// how these tests get a second source to delete or activate.
func CreateBookSource(t *testing.T, env *Env, bookID string) string {
	t.Helper()

	rec := env.Post(BookURL(bookID, "sources"), nil)
	AssertStatus(t, rec, http.StatusOK)
	AssertJSONContentType(t, rec)

	created := DecodeJSON[map[string]any](t, rec)
	sourceID, _ := created["id"].(string)
	if sourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", created)
	}
	return sourceID
}

// SourceIDs lists the IDs the sources endpoint reports for a book.
func SourceIDs(t *testing.T, env *Env, bookID string) []string {
	t.Helper()

	var ids []string
	for _, source := range GetJSON[[]map[string]any](t, env, BookURL(bookID, "sources")) {
		id, _ := source["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

// CurrentSourceOf reports the pointer the book endpoint publishes, which is what
// a client reads back after a source is deleted.
func CurrentSourceOf(t *testing.T, env *Env, bookID string) string {
	t.Helper()

	book := GetJSON[server.Book](t, env, BookURL(bookID))
	if book.Meta == nil {
		t.Fatalf("book %s carried no meta", bookID)
	}
	return book.Meta.CurrentSource
}
