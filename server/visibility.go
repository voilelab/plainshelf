package server

import (
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// bookVisibility answers which of a shelf's books exist at all, as far as one
// request is concerned.
//
// One type rather than a check per handler because the answer has to be the same
// on every route: a listing that filtered while the single-book route did not
// would still serve the book to anyone holding its URL. Listings go through
// listBooks, and a request naming one book through apiCore.lookupBookListing.
//
// The rule itself belongs to the shelf - Shelf.IsBookNSFW - because the mark
// travels with the files and every reader has to compute it the same way. What
// this adds is the server's decision about whether a marked book is served.
type bookVisibility struct {
	shelf *shelf.Shelf

	// showNSFW is read once, when the filter is built, so a listing and the
	// folder filter derived from it cannot disagree because the setting was
	// changed halfway through a request.
	showNSFW bool
}

// visibility resolves the filter for one request against one shelf.
func (c *apiCore) visibility(shelfData *shelf.ShelfData) bookVisibility {
	return bookVisibility{shelf: shelfData.Shelf, showNSFW: c.settings.showNSFW()}
}

// allows reports whether a book sitting at folders, carrying meta, is part of
// the shelf this request sees.
func (v bookVisibility) allows(folders shelf.FolderPath, meta *shelf.BookMeta) bool {
	return v.showNSFW || !v.shelf.IsBookNSFW(folders, meta)
}

func (v bookVisibility) allowsListing(listing shelf.BookListing) bool {
	return v.allows(listing.Folders, listing.Book.GetMeta())
}

// listBooks is the shelf's listing with the books this request may not see
// removed. It is what every endpoint that returns more than one book lists
// through.
func (v bookVisibility) listBooks() ([]shelf.BookListing, error) {
	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return v.keep(listings), nil
}

func (v bookVisibility) keep(listings []shelf.BookListing) []shelf.BookListing {
	if v.showNSFW {
		return listings
	}

	kept := make([]shelf.BookListing, 0, len(listings))
	for _, listing := range listings {
		if v.allowsListing(listing) {
			kept = append(kept, listing)
		}
	}
	return kept
}

// visibleBookIDs is the set a background sweep started by this request is
// limited to, or nil when the request sees everything - which leaves such a
// sweep exactly as it was before this setting existed.
//
// It is a snapshot: the sweep runs later, so a book added in between is left to
// the next one rather than swept without anyone having decided it is visible.
func (v bookVisibility) visibleBookIDs() (map[string]bool, error) {
	if v.showNSFW {
		return nil, nil
	}

	listings, err := v.listBooks()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	ids := make(map[string]bool, len(listings))
	for _, listing := range listings {
		ids[listing.Book.ID()] = true
	}
	return ids, nil
}

// filterFolders drops the folders this request must not see, keeping the rest in
// the order they were given.
//
// Two kinds go: a folder inside a marked subtree, and a folder whose books are
// all hidden. The second stops the mark showing through its own absence - a
// folder holding nothing but marked books would otherwise stay in the tree, the
// breadcrumbs and the move-book menu, and its name is usually the disclosure.
//
// A folder holding no books at all is kept: someone made it, and dropping it
// would take away a destination the user needs. Ancestors follow from the same
// rule, since a folder holding a visible book anywhere below it counts.
func (v bookVisibility) filterFolders(folders []shelf.FolderPath) ([]shelf.FolderPath, error) {
	if v.showNSFW {
		return folders, nil
	}

	// The unfiltered listing is the half that says whether a folder is empty
	// only because its books are hidden; the filtered one alone cannot tell that
	// from a folder that was always empty.
	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	holdsBook := map[string]bool{}
	holdsVisibleBook := map[string]bool{}
	for _, listing := range listings {
		allowed := v.allowsListing(listing)
		// A book counts for its own folder and for every folder above it, so a
		// parent is judged by everything in its subtree.
		for depth := range len(listing.Folders) + 1 {
			key := listing.Folders[:depth].String()
			holdsBook[key] = true
			if allowed {
				holdsVisibleBook[key] = true
			}
		}
	}

	kept := make([]shelf.FolderPath, 0, len(folders))
	for _, folder := range folders {
		if v.shelf.IsNSFWFolder(folder) {
			continue
		}
		if key := folder.String(); holdsBook[key] && !holdsVisibleBook[key] {
			continue
		}
		kept = append(kept, folder)
	}
	return kept, nil
}
