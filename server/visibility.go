package server

import (
	"slices"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/task"
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
	return bookVisibility{shelf: shelfData.Shelf, showNSFW: c.settings.readShowNSFW()}
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

func (v bookVisibility) visibleBooks() (task.VisibleBooks, error) {
	if v.showNSFW {
		return task.AllBooks(), nil
	}

	listings, err := v.listBooks()
	if err != nil {
		return task.VisibleBooks{}, util.Errorf("%w", err)
	}

	ids := make([]string, 0, len(listings))
	for _, listing := range listings {
		ids = append(ids, listing.Book.ID())
	}
	return task.OnlyBooks(ids...), nil
}

type occupiedFolders map[string]bool

func (o occupiedFolders) add(folders shelf.FolderPath) {
	for depth := range len(folders) + 1 {
		o[folders[:depth].String()] = true
	}
}

func (o occupiedFolders) holds(folder shelf.FolderPath) bool {
	return o[folder.String()]
}

func (v bookVisibility) filterFolders(folders []shelf.FolderPath) ([]shelf.FolderPath, error) {
	if v.showNSFW {
		return folders, nil
	}

	listings, err := v.shelf.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	holdsBook := occupiedFolders{}
	holdsVisibleBook := occupiedFolders{}
	for _, listing := range listings {
		holdsBook.add(listing.Folders)
		if v.allowsListing(listing) {
			holdsVisibleBook.add(listing.Folders)
		}
	}

	kept := make([]shelf.FolderPath, 0, len(folders))
	for _, folder := range folders {
		if v.shelf.IsNSFWFolder(folder) {
			continue
		}
		if holdsBook.holds(folder) && !holdsVisibleBook.holds(folder) {
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
	From        shelf.FolderPath
	To          shelf.FolderPath
	leavesShelf bool
}

func folderMovedTo(from, to shelf.FolderPath) folderChange {
	return folderChange{From: from, To: to}
}

func folderLeavingTheShelf(from shelf.FolderPath) folderChange {
	return folderChange{From: from, leavesShelf: true}
}

func (c folderChange) stillMarked(s *shelf.Shelf, folders shelf.FolderPath, meta *shelf.BookMeta) bool {
	if c.leavesShelf {
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
	holdsBook := occupiedFolders{}
	for _, listing := range listings {
		if !listing.Folders.HasPrefix(change.From) {
			continue
		}
		holdsBook.add(listing.Folders)

		meta := listing.Book.GetMeta()
		if !v.allows(listing.Folders, meta) && !change.stillMarked(v.shelf, listing.Folders, meta) {
			reveal.Books++
		}
	}

	folders, err := v.shelf.GetAllFolders()
	if err != nil {
		return folderReveal{}, util.Errorf("%w", err)
	}

	if !change.leavesShelf && slices.ContainsFunc(folders, change.To.Equal) {
		return folderReveal{}, nil
	}

	for _, sub := range folders {
		if !sub.HasPrefix(change.From) || !v.shelf.IsNSFWFolder(sub) {
			continue
		}
		if change.stillMarked(v.shelf, sub, nil) {
			continue
		}

		if !holdsBook.holds(sub) {
			reveal.Folders++
		}
	}

	return reveal, nil
}
