package server

import (
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

var openFinder = util.OpenFinder

// OpenBookFolder opens the local filesystem folder of a book.
// This is intended for desktop application use.
func (app *App) OpenBookFolder(shelfID, bookID string) error {
	targetShelfID := strings.TrimSpace(shelfID)
	if targetShelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	targetBookID := strings.TrimSpace(bookID)
	if targetBookID == "" {
		return util.Errorf("book ID cannot be empty")
	}

	shelfData, ok := app.shelfManager.GetShelf(targetShelfID)
	if !ok {
		return util.Errorf("shelf not found: %s", targetShelfID)
	}

	book, err := shelfData.GetBook(targetBookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := openFinder(book.FolderPath()); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}
