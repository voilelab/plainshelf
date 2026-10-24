package task

import "github.com/voilelab/plainshelf/shelf"

type VisibleBooks struct {
	all bool
	ids map[string]bool
}

func AllBooks() VisibleBooks {
	return VisibleBooks{all: true}
}

func OnlyBooks(ids ...string) VisibleBooks {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return VisibleBooks{ids: set}
}

func (v VisibleBooks) Allows(bookID string) bool {
	return v.all || v.ids[bookID]
}

func (v VisibleBooks) Only(books []*shelf.Book) []*shelf.Book {
	if v.all {
		return books
	}

	kept := make([]*shelf.Book, 0, len(books))
	for _, book := range books {
		if v.Allows(book.ID()) {
			kept = append(kept, book)
		}
	}
	return kept
}
