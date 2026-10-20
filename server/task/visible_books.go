package task

import "github.com/voilelab/plainshelf/shelf"

type VisibleBooks map[string]bool

func (v VisibleBooks) Allows(bookID string) bool {
	return v == nil || v[bookID]
}

func (v VisibleBooks) Only(books []*shelf.Book) []*shelf.Book {
	if v == nil {
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
