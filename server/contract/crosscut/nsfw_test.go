package crosscut_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

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

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	for _, bookID := range s.Hidden() {
		apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(bookID)), http.StatusOK)
		apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(bookID, "content")), http.StatusOK)
	}
}

// A shelf given its own answer keeps it whatever the setting says, in the
// listing and the folder tree alike, and goes back to the setting when cleared.
func TestAPINSFWShelfAnswerOverridesTheSetting(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	folders := func() []shelf.FolderPath {
		return apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders"))
	}
	hiddenFolders := folders()

	s.Env.App.SetShelfShowNSFW(apitest.DefaultShelfID, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	if got := folders(); len(got) <= len(hiddenFolders) {
		t.Errorf("folders with the shelf shown = %v, want more than %v", got, hiddenFolders)
	}

	apitest.SetShowNSFW(t, s.Env, true)
	s.Env.App.SetShelfShowNSFW(apitest.DefaultShelfID, false)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)

	s.Env.App.ClearShelfShowNSFW(apitest.DefaultShelfID)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
}
