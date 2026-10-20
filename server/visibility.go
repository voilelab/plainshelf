package server

import (
	"slices"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type bookVisibility struct {
	shelf *shelf.Shelf

	showNSFW bool
}

func (c *apiCore) visibility(shelfData *shelf.ShelfData) bookVisibility {
	if show, ok := c.showNSFWByShelf.Load(shelfData.ID); ok {
		return bookVisibility{shelf: shelfData.Shelf, showNSFW: show.(bool)}
	}
	return bookVisibility{shelf: shelfData.Shelf, showNSFW: c.settings.showNSFW()}
}

func (v bookVisibility) allows(folders shelf.FolderPath, meta *shelf.BookMeta) bool {
	return v.showNSFW || !v.shelf.IsBookNSFW(folders, meta)
}

func (v bookVisibility) allowsListing(listing shelf.BookListing) bool {
	return v.allows(listing.Folders, listing.Book.GetMeta())
}

func (v bookVisibility) allowsTrashed(book *shelf.TrashedBook) bool {
	return v.showNSFW || !book.NSFW
}

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

func (v bookVisibility) filterFolders(folders []shelf.FolderPath) ([]shelf.FolderPath, error) {
	if v.showNSFW {
		return folders, nil
	}

	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	holdsBook := map[string]bool{}
	holdsVisibleBook := map[string]bool{}
	for _, listing := range listings {
		allowed := v.allowsListing(listing)

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

type folderReveal struct {
	Books   int
	Folders int
}

func (r folderReveal) any() bool { return r.Books > 0 || r.Folders > 0 }

type folderChange struct {
	From shelf.FolderPath
	To   shelf.FolderPath
}

func folderMovedTo(from, to shelf.FolderPath) folderChange {
	return folderChange{From: from, To: to}
}

func folderLeavingTheShelf(from shelf.FolderPath) folderChange {
	return folderChange{From: from}
}

func (c folderChange) stillMarked(s *shelf.Shelf, folders shelf.FolderPath, meta *shelf.BookMeta) bool {
	if c.To == nil {

		return meta != nil && meta.NSFW
	}
	moved := slices.Concat(c.To, folders[len(c.From):])
	return s.IsBookNSFW(moved, meta)
}

func (v bookVisibility) couldReveal() bool {
	return !v.showNSFW && v.shelf.MarksFolders()
}

func (v bookVisibility) revealedBy(change folderChange) (folderReveal, error) {
	if !v.couldReveal() {
		return folderReveal{}, nil
	}

	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return folderReveal{}, util.Errorf("%w", err)
	}

	var reveal folderReveal

	holdsBook := map[string]bool{}
	for _, listing := range listings {
		if !listing.Folders.HasPrefix(change.From) {
			continue
		}
		for depth := range len(listing.Folders) + 1 {
			holdsBook[listing.Folders[:depth].String()] = true
		}

		meta := listing.Book.GetMeta()
		if !v.allows(listing.Folders, meta) && !change.stillMarked(v.shelf, listing.Folders, meta) {
			reveal.Books++
		}
	}

	folders, err := v.shelf.GetAllFolders()
	if err != nil {
		return folderReveal{}, util.Errorf("%w", err)
	}

	if change.To != nil && slices.ContainsFunc(folders, change.To.Equal) {
		return folderReveal{}, nil
	}

	for _, sub := range folders {
		if !sub.HasPrefix(change.From) || !v.shelf.IsNSFWFolder(sub) {
			continue
		}
		if change.stillMarked(v.shelf, sub, nil) {
			continue
		}

		if !holdsBook[sub.String()] {
			reveal.Folders++
		}
	}

	return reveal, nil
}
