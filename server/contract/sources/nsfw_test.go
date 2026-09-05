package sources_test

import (
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

// The similarity endpoints and the sweep that feeds them cover only the books a
// request may see. The coverage count matters as much as the results: a total
// taken over the whole shelf reports that books exist even when the pairs beside
// it are empty. See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPINSFWBooksAreAbsentFromSimilarityAndItsCount(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	for _, bookID := range s.All() {
		backdateSources(t, s.Env, bookID)
	}

	pairedIDs := func() []string {
		ids := []string{}
		for _, pair := range apitest.GetJSON[[]similarPairResponse](t, s.Env, apitest.SimilarURL()) {
			ids = append(ids, pair.A, pair.B)
		}
		return ids
	}
	total := func() int {
		return apitest.GetJSON[fingerprintStatusResponse](t, s.Env, apitest.FingerprintStatusURL()).Total
	}

	// Fingerprint everything first, so an absent pair below is the filter rather
	// than a fingerprint that was never built.
	apitest.SetShowNSFW(t, s.Env, true)
	if got := runFingerprintSources(t, s.Env).Books; got != len(s.All()) {
		t.Fatalf("sweep covered %d books with show_nsfw on, want all %d", got, len(s.All()))
	}
	apitest.AssertBookIDs(t, pairedIDs(), s.Visible, s.FolderHidden)
	if got := total(); got != len(s.All()) {
		t.Errorf("coverage counted %d books with show_nsfw on, want all %d", got, len(s.All()))
	}

	apitest.SetShowNSFW(t, s.Env, false)
	// The sweep runs after the response that started it, so what it may read is
	// decided at request time; its book count is the observable half.
	if got := runFingerprintSources(t, s.Env).Books; got != 2 {
		t.Errorf("sweep covered %d books, want the 2 this request can see", got)
	}
	if got := pairedIDs(); len(got) != 0 {
		t.Errorf("paired IDs = %v, want none: the only match for Visible is hidden", got)
	}
	if got := total(); got != 2 {
		t.Errorf("coverage counted %d books, want the 2 this request can see", got)
	}
}
