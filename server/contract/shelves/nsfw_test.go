package shelves_test

import (
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

// The exported cache mirrors the shelf, not this machine's display setting. The
// client reading it applies its own — the mark is in every entry for exactly
// that reason — and a cache pruned here would quietly become that client's whole
// library. See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPIBookCacheExportIsCompleteWhateverShowNSFWSays(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	exportedIDs := func() []string {
		apitest.AssertStatus(t, s.Env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)

		matches, err := filepath.Glob(filepath.Join(s.Env.LibRoot, "app", "book-cache-*.json"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("glob for the exported cache = %v (%v), want exactly one file", matches, err)
		}
		raw, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatalf("read %s: %v", matches[0], err)
		}
		var file struct {
			Books map[string]struct {
				NSFW bool `json:"nsfw"`
			} `json:"books"`
		}
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("decoding %s: %v", matches[0], err)
		}

		ids := []string{}
		for id, entry := range file.Books {
			ids = append(ids, id)
			if want := slices.Contains(s.Hidden(), id); entry.NSFW != want {
				t.Errorf("exported nsfw for %s = %v, want %v", id, entry.NSFW, want)
			}
		}
		return ids
	}

	apitest.AssertBookIDs(t, exportedIDs(), s.All()...)
	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, exportedIDs(), s.All()...)
}
