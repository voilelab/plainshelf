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

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listBooksFromCache(), nil
}

// GetBook returns the details of a specific book by its ID.
func (s *Shelf) GetBook(bookID string) (*Book, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return book, nil
}

// NewBook creates a new book with the given ID and title, and returns the created Book instance.
// It is an atomic operation that ensures the book is fully created before it becomes visible in the library.
func (s *Shelf) NewBook(layers Layers, title string) (*Book, error) {
	return s.NewBookWith(layers, title, nil)
}

// NewBookWith creates a new book and, if init is non-nil, runs it against the
// book while the book is still staged in the app temp folder.
//
// The book only becomes visible under the books folder after init succeeds, so
// a failing init - or a crash partway through it - leaves nothing behind but
// temp data, which is wiped on the next startup. This is what makes multi-step
// creation (book, source, current-source pointer, metadata) transactional.
//
// init runs while the exclusive shelf lock is held, so it should not perform
// long-running work beyond writing the book's own initial content.
func (s *Shelf) NewBookWith(layers Layers, title string, init func(*Book) error) (*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	bookPath, err := createTempDir(s.dbRoot, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.dbRoot.RemoveAll(bookPath)

	// The ID is drawn at random, not derived from the layers and title: what
	// keeps two books apart is the entropy behind it, not the probe below. The
	// probe only sees books this process already knows about - the cache does not
	// notice a book another machine added to a shared shelf, or one copied in
	// with a file manager, until it rescans - so it is insurance against an ID
	// this shelf demonstrably holds, and is expected never to fire.
	bookID := ""
	for range MaxBookIDCreationAttempts {
		candidate, idErr := newBookID()
		if idErr != nil {
			return nil, util.Errorf("%w", idErr)
		}

		_, err := s.getUpdatedBookFromBookID(candidate)
		if errors.Is(err, ErrBookNotFound) {
			inTrash, trashErr := s.isBookIDInTrash(candidate)
			if trashErr != nil {
				return nil, util.Errorf("%w", trashErr)
			}
			if !inTrash {
				bookID = candidate
				break
			}
		} else if err != nil {
			return nil, util.Errorf("%w", err)
		}
	}
	if bookID == "" {
		return nil, util.NewError("failed to draw an unused book ID after multiple attempts")
	}

	stagedBook, err := createBook(s.dbRoot, s.Logger, bookPath, bookID, title)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Everything init writes lands inside the staging folder, which the deferred
	// RemoveAll above discards if init fails.
	if init != nil {
		if err := init(stagedBook); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	layerPath := path.Join(booksFolder, path.Join(layers...))

	err = s.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Creating a book can create layers on the way, and the layer listing is
	// served from the cache; record them now rather than at the next scan.
	s.addLayersToBookCache(layers)

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

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

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

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newLayerPath := path.Join(booksFolder, path.Join(newLayers...))
	err = s.dbRoot.MkdirAll(newLayerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.addLayersToBookCache(newLayers)

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

// iterateShelfTree walks the books folder once, reporting every layer
// directory to onLayer and every book package to onBook. Either callback may be
// nil. Returning false from a callback stops the whole walk. It returns what
// the walk cost, which is what makes the effect of the scan cache visible.
//
// Books and layers share one walk because they are answers to the same
// question: a listing and a layer tree both describe the shape of books/, and
// walking it twice is the cost this shelf can least afford on a network mount.
// scanToBookCache is the only production caller for that reason.
func (s *Shelf) iterateShelfTree(onLayer func(Layers) bool, onBook func(*Book) bool) (scanStats, error) {
	skipAll := false

	// The walk reads each directory through the scan cache, which turns a
	// ReadDir into a Stat for every folder that has not changed since the last
	// walk. See shelf_scan_cache.go.
	dirCache := s.newScanDirCache()

	var dfsFunc func(string, *dirChild)

	dfsFunc = func(pth string, child *dirChild) {
		if skipAll {
			return
		}

		isDir, err := childIsDir(s.dbRoot, pth, child)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to stat path during book scan", "path", pth, "error", err)
			}
			return
		}

		if !isDir {
			return
		}

		folderName := path.Base(pth)
		if isIgnoredDir(folderName) {
			return
		}

		// Paths are always built with path.Join, which uses "/" on every
		// platform, so split on "/" rather than os.PathSeparator (which would
		// be "\" on Windows and break layer parsing).
		if strings.HasSuffix(folderName, bookExtension) {
			if onBook == nil {
				return
			}

			book, err := openBook(s.dbRoot, s.Logger, pth)
			if err != nil {
				s.Error("Error opening book", "path", pth, "error", err)
				return
			}

			layers := strings.Split(path.Dir(pth), "/")[1:]
			book.setLayers(layers)

			if !onBook(book) {
				skipAll = true
			}
			return
		}

		if onLayer != nil && !onLayer(strings.Split(pth, "/")[1:]) {
			skipAll = true
			return
		}

		children, err := dirCache.readDir(pth)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to read directory during book scan", "path", pth, "error", err)
			}
			return
		}

		for i := range children {
			fullPath := path.Join(pth, children[i].Name)
			dfsFunc(fullPath, &children[i])
		}
	}

	dfsFunc(booksFolder, nil)
	return dirCache.install(!skipAll), nil
}
