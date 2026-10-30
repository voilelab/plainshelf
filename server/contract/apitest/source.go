package apitest

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

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

func SourceIDs(t *testing.T, env *Env, bookID string) []string {
	t.Helper()

	var ids []string
	for _, source := range GetJSON[[]map[string]any](t, env, BookURL(bookID, "sources")) {
		id, _ := source["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func CurrentSourceOf(t *testing.T, env *Env, bookID string) string {
	t.Helper()

	book := GetJSON[server.Book](t, env, BookURL(bookID))
	if book.Meta == nil {
		t.Fatalf("book %s carried no meta", bookID)
	}
	return book.Meta.CurrentSource
}
