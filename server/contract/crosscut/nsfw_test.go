package crosscut_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

/*
show_nsfw decides whether this server serves the books its shelves mark as adult
content, and defaults to off. The filtering is server-side: a title or a cover
that reached the browser to be hidden there has already been in the network
trace and the cache.

What is asserted here is the part every route owes whatever else it does — the
listing and the answer to a book named directly. The rest of the surface is
pinned in its own area against the same fixture (apitest.NewNSFWShelf): folders
in folders/, similarity and its coverage count in sources/, batches in books/,
the exported cache in shelves/, and the setting's own shape in platform/.

Every test carries its reverse half: with the setting on, the same requests
answer exactly as they did before the setting existed.
*/

// A marked book is in no listing, and neither is the fact that it has a twin.
func TestAPINSFWBooksAreAbsentFromListings(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	duplicates := func() [][]string {
		return apitest.GetJSON[[][]string](t, s.Env, apitest.ShelfURL("books", "duplicate"))
	}

	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)
	if groups := duplicates(); len(groups) != 0 {
		t.Errorf("duplicate groups = %v, want none while the twin is hidden", groups)
	}

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	groups := duplicates()
	if len(groups) != 1 {
		t.Fatalf("duplicate groups = %v, want the one pair", groups)
	}
	apitest.AssertBookIDs(t, groups[0], s.Visible, s.FolderHidden)
}

// Every route naming one book answers as though it were not there: 403 would
// confirm it exists. An ID that was never issued is held to the same envelope,
// which is the claim — the two answers are indistinguishable.
func TestAPINSFWBooksAnswerNotFoundWhereverTheyAreNamed(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	// A book is refused before its source is looked for, so this real ID from
	// another book is enough to address the source route.
	sourceID := apitest.CurrentSourceOf(t, s.Env, s.Visible)

	routes := []struct {
		method string
		elem   []string
		body   string
	}{
		{method: http.MethodGet},
		{method: http.MethodPatch, body: `{"title":"renamed"}`},
		{method: http.MethodDelete},
		{method: http.MethodGet, elem: []string{"cover"}},
		{method: http.MethodGet, elem: []string{"content"}},
		{method: http.MethodGet, elem: []string{"sources"}},
		{method: http.MethodPost, elem: []string{"copies"}},
		{method: http.MethodGet, elem: []string{"sources", sourceID, "content"}},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+strings.Join(tc.elem, "/"), func(t *testing.T) {
			for _, bookID := range append([]string{"no_such_book"}, s.Hidden()...) {
				var body io.Reader
				if tc.body != "" {
					body = strings.NewReader(tc.body)
				}
				rec := s.Env.Request(tc.method, apitest.BookURL(bookID, tc.elem...), body)
				apitest.AssertErrorEnvelope(t, rec, http.StatusNotFound, "BOOK_NOT_FOUND", "book not found")
			}
		})
	}

	// Nothing above deleted anything: the refusals were the filter, and the same
	// books answer normally once they may be seen.
	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	for _, bookID := range s.Hidden() {
		apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(bookID)), http.StatusOK)
		apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(bookID, "content")), http.StatusOK)
	}
}
