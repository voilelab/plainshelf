package server

import (
	"slices"

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

// allowsTrashed is the same question for a book sitting in the trash.
//
// Deleting a book must not disclose it: the trash listing is a listing like any
// other, and its titles are the disclosure. Both halves of the mark still
// apply, because the trash record remembers the folder the book was deleted
// from — so the shelf answers with TrashedBook.NSFW rather than this walking a
// path that no longer exists. Emptying the trash is not a listing and is not
// filtered; it is one command over the whole trash.
func (v bookVisibility) allowsTrashed(book *shelf.TrashedBook) bool {
	return v.showNSFW || !book.NSFW
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

// folderReveal is what one folder change would take out of a marked subtree, as
// far as this request is concerned.
//
// Books and folders are counted apart because either alone is a disclosure: a
// book hidden by a folder rule is a title this request has never been served,
// and a folder that becomes listed names what it holds.
type folderReveal struct {
	Books   int
	Folders int
}

func (r folderReveal) any() bool { return r.Books > 0 || r.Folders > 0 }

// folderChange is one folder move or rename, seen from the shelf it happens on.
// To is where the folder lands; a change built by folderLeavingTheShelf has
// none, which is what a cross-shelf transfer does.
type folderChange struct {
	From shelf.FolderPath
	To   shelf.FolderPath
}

// folderMovedTo is a move or a rename within one shelf. A rename is a move that
// keeps the parent, so both are the same question: everything under From lands
// under To, and this shelf's own rules decide the result.
func folderMovedTo(from, to shelf.FolderPath) folderChange {
	return folderChange{From: from, To: to}
}

// folderLeavingTheShelf is a cross-shelf transfer, for a copy as well as a move:
// a copy leaves this shelf untouched but publishes the same titles on the other
// one, which is the same disclosure.
func folderLeavingTheShelf(from shelf.FolderPath) folderChange {
	return folderChange{From: from}
}

// stillMarked answers, for one folder currently under From and one book sitting
// in it, whether it is still marked once the change is done. A nil meta asks
// about the folder itself, which IsBookNSFW answers as IsNSFWFolder.
func (c folderChange) stillMarked(s *shelf.Shelf, folders shelf.FolderPath, meta *shelf.BookMeta) bool {
	if c.To == nil {
		// Only a book's own nsfw travels off this shelf, because it is written
		// in its book.json. shelf.json stays behind, and whether the target
		// shelf happens to mark the same path is the target's business - reading
		// it here would let a coincidence of naming decide what this shelf gives
		// up.
		return meta != nil && meta.NSFW
	}
	moved := append(append(shelf.FolderPath(nil), c.To...), folders[len(c.From):]...)
	return s.IsBookNSFW(moved, meta)
}

// couldReveal is the cheap half of "would this change disclose anything": with
// the setting on there is nothing hidden to reveal, and a shelf that marks no
// folder can never lose a mark by moving one. Everything below walks the shelf,
// so this is what keeps that walk off the shelves it cannot tell anything about.
func (v bookVisibility) couldReveal() bool {
	return !v.showNSFW && v.shelf.MarksFolders()
}

// revealedBy counts what change stops this shelf from marking.
func (v bookVisibility) revealedBy(change folderChange) (folderReveal, error) {
	if !v.couldReveal() {
		return folderReveal{}, nil
	}

	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return folderReveal{}, util.Errorf("%w", err)
	}

	var reveal folderReveal

	// holdsBook is filterFolders' own question, asked over the subtree that is
	// about to move: a book counts for its folder and for every folder above it.
	// The books travel with the folder, so the answer is the same afterwards.
	holdsBook := map[string]bool{}
	for _, listing := range listings {
		if !folderHasPrefix(listing.Folders, change.From) {
			continue
		}
		for depth := range len(listing.Folders) + 1 {
			holdsBook[listing.Folders[:depth].String()] = true
		}
		// Already visible, or marked by something the change does not touch:
		// neither is a book this request would newly be served.
		meta := listing.Book.GetMeta()
		if !v.allows(listing.Folders, meta) && !change.stillMarked(v.shelf, listing.Folders, meta) {
			reveal.Books++
		}
	}

	folders, err := v.shelf.GetAllFolders()
	if err != nil {
		return folderReveal{}, util.Errorf("%w", err)
	}

	// A change that cannot run must fail as itself rather than as a disclosure
	// the user approves for nothing: the shelf refuses to land a folder on a
	// name it already holds. Its own check, under its own lock, is the
	// authority; this only declines to ask first when the same listing already
	// shows the change going nowhere.
	if change.To != nil && slices.ContainsFunc(folders, change.To.Equal) {
		return folderReveal{}, nil
	}

	for _, sub := range folders {
		if !folderHasPrefix(sub, change.From) || !v.shelf.IsNSFWFolder(sub) {
			continue
		}
		if change.stillMarked(v.shelf, sub, nil) {
			continue
		}
		// Losing the mark is not enough to become visible. filterFolders drops a
		// folder whose books are all filtered out, so one still holding nothing
		// but hidden books stays out of the tree on its own account. Only a
		// folder holding no book at all is disclosed by its name alone - and any
		// book of its that does become visible is counted above.
		if !holdsBook[sub.String()] {
			reveal.Folders++
		}
	}

	return reveal, nil
}
