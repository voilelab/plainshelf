package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// ListBooks returns a list of all books in the library.
// Books are sorted by their ID in ascending order.
func (s *Shelf) ListBooks() ([]*Book, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.rlock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.unlock()

	err := s.refreshBookCacheIfNeeded(false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return s.listBooksFromCache(), nil
}

// GetBook returns the details of a specific book by its ID.
func (s *Shelf) GetBook(bookID string) (*Book, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.rlock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return book, nil
}

// NewBook creates a new book with the given ID and title, and returns the created Book instance.
// It is an atomic operation that ensures the book is fully created before it becomes visible in the library.
func (s *Shelf) NewBook(layers Layers, title string) (*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.unlock()

	bookPath, err := createTempDir(s.dbRoot, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.dbRoot.RemoveAll(bookPath)

	// Generate a unique book ID based on the layers and title
	// TBD: Use UUID
	baseBookID := generateBookID(layers, title)
	bookID := baseBookID
	for i := 1; ; i++ {
		_, err := s.getUpdatedBookFromBookID(bookID)
		if errors.Is(err, ErrBookNotFound) {
			inTrash, trashErr := s.isBookIDInTrash(bookID)
			if trashErr != nil {
				return nil, util.Errorf("%w", trashErr)
			}
			if !inTrash {
				break
			}
		} else if err != nil {
			return nil, util.Errorf("%w", err)
		}
		bookID = fmt.Sprintf("%s-%d", baseBookID, i)
	}

	_, err = createBook(s.dbRoot, s.Logger, bookPath, bookID, title)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	layerPath := path.Join(booksFolder, path.Join(layers...))

	err = s.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	folderName := titleToFolderName(title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(layerPath, folderName)
		if _, err := s.dbRoot.Stat(finalBookPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		} else {
			folderName = titleToFolderName(fmt.Sprintf("%s-%d", title, i))
		}
	}

	finalBookPath := path.Join(layerPath, folderName)
	err = s.dbRoot.Rename(bookPath, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := openBook(s.dbRoot, s.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook.setLayers(layers)

	s.updateBookCacheEntry(layers, finalBookPath, newBook)

	return newBook, nil
}

// DeleteBook moves a book into trash by its ID.
func (s *Shelf) DeleteBook(bookID string) error {
	return s.MoveBookToTrash(bookID)
}

// GetBooksByLayer returns a list of books that belong to the specified layers.
func (s *Shelf) GetBooksByLayer(layers Layers) ([]*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.rlock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.unlock()

	err := s.refreshBookCacheIfNeeded(false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var books []*Book

	for _, book := range s.listBooksFromCache() {
		if book.Layers().Equal(layers) {
			books = append(books, book)
		}
	}

	return books, nil
}

// MoveBook moves a book to new layers and returns the updated Book instance.
func (s *Shelf) MoveBook(bookID string, newLayers Layers) (*Book, error) {
	if err := validateLayers(newLayers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newLayerPath := path.Join(booksFolder, path.Join(newLayers...))
	err = s.dbRoot.MkdirAll(newLayerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBookPath := path.Join(newLayerPath, path.Base(book.FolderPath()))
	err = s.dbRoot.Rename(book.FolderPath(), newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook, err := openBook(s.dbRoot, s.Logger, newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook.setLayers(newLayers)

	s.updateBookCacheEntry(newLayers, newBookPath, movedBook)

	return movedBook, nil
}

// iterateBooks iterates over all books under the specified layers and applies the provided function to each book.
// If the function returns false, the iteration will stop.
func (s *Shelf) iterateBooks(rLayers Layers, fn func(*Book) bool) error {
	visitFolder := path.Join(booksFolder, path.Join(rLayers...))

	skipAll := false

	var dfsFunc func(string)

	dfsFunc = func(pth string) {
		if skipAll {
			return
		}

		stat, err := s.dbRoot.Stat(pth)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to stat path during book scan", "path", pth, "error", err)
			}
			return
		}

		if !stat.IsDir() {
			return
		}

		folderName := path.Base(pth)
		if strings.HasSuffix(folderName, bookExtension) {
			book, err := openBook(s.dbRoot, s.Logger, pth)
			if err != nil {
				s.Error("Error opening book", "path", pth, "error", err)
				return
			}

			layers := strings.Split(path.Dir(pth), string(os.PathSeparator))[1:]
			book.setLayers(layers)

			if !fn(book) {
				skipAll = true
			}
			return
		}

		entries, err := s.dbRoot.ReadDir(pth)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to read directory during book scan", "path", pth, "error", err)
			}
			return
		}

		for _, entry := range entries {
			fullPath := path.Join(pth, entry.Name())
			dfsFunc(fullPath)
		}
	}

	dfsFunc(visitFolder)
	return nil
}
