package folders_test

import (
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/shelf"
)

// A folder goes out of the tree with its books, so a name that is usually the
// whole disclosure cannot survive in a breadcrumb or in the destination list
// when moving a book. See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPINSFWFoldersLeaveTheTreeWithTheirBooks(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	folders := func() []string {
		paths := []string{}
		for _, folder := range apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders")) {
			paths = append(paths, folder.String())
		}
		return paths
	}
	assertListed := func(got []string, want bool, folders ...string) {
		t.Helper()
		for _, folder := range folders {
			if slices.Contains(got, folder) != want {
				t.Errorf("folders = %v, want %q listed = %v", got, folder, want)
			}
		}
	}

	got := folders()
	// Fiction still holds Classic, and Empty never held a book at all, so
	// neither is this setting's to remove.
	assertListed(got, true, "Fiction", "Fiction/Classics", apitest.NSFWEmptyFolder)
	// Marked, and left holding only a marked book, respectively.
	assertListed(got, false, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)

	apitest.SetShowNSFW(t, s.Env, true)
	assertListed(folders(), true, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)
}
