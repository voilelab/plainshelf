package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

// BookListing is one book of a listing together with the values the book cache
// keeps beside it.
type BookListing struct {
	Book *Book

	// Layers is where the book sits in the shelf tree, taken from its book cache
	// entry. The Book itself does not carry its layer — a layer is a position in
	// the tree, not part of the book's identity — so the listing is how a caller
	// learns it. Empty for a book at the top level of books/.
	Layers Layers

	// CharCount is the character count of the book's current source as of the
	// last time its cache entry was built, and 0 when the book has no readable
	// current source. See Shelf.RefreshBookCharCount for what keeps it current.
	CharCount int
}

// ListBooks returns a list of all books in the library.
// Books are sorted by their ID in ascending order.
func (s *Shelf) ListBooks() ([]*Book, error) {
	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var books []*Book
	for _, listing := range listings {
		books = append(books, listing.Book)
	}
	return books, nil
}

// ListBooksWithCharCount is ListBooks plus each book's character count, taken
// from the book cache rather than from the shelf.
//
// It exists so that a listing can report character counts without opening one
// source meta.json per book on the request path; see bookIDCacheEntry.charCount
// for why that cost does not belong there.
func (s *Shelf) ListBooksWithCharCount() ([]BookListing, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listBookListingsFromCache(), nil
}

func (s *Shelf) GetBook(bookID string) (*Book, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, _, err := s.getUpdatedBookFromBookID(bookID)
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
// book while it is still staged in the app temp folder.
//
// The book becomes visible under the books folder only after init succeeds, so
// a failing init - or a crash partway through it - leaves nothing but temp
// data, wiped on the next startup. This is what makes multi-step creation
// (book, source, current-source pointer, metadata) transactional.
//
// init runs while the exclusive shelf lock is held, so it should not do
// long-running work beyond writing the book's own initial content.
func (s *Shelf) NewBookWith(layers Layers, title string, init func(*Book) error) (*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	bookPath, err := createTempDir(root, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer root.RemoveAll(bookPath)

	bookID, err := s.drawUnusedBookID()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	stagedBook, err := bookpkg.Create(root, s.Logger, bookPath, bookID, title)
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

	err = root.MkdirAll(layerPath)
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
	err = root.Rename(bookPath, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := bookpkg.Open(s.dbRoot, s.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.updateBookCacheEntry(layers, finalBookPath, newBook)

	return newBook, nil
}

// GetBookListing returns a book together with the layer it sits in.
//
// GetBook answers with the book alone; this is for a caller that also needs the
// book's position, which the book itself does not carry. The layer comes from
// the book cache entry, the single owner of that fact.
func (s *Shelf) GetBookListing(bookID string) (BookListing, error) {
	if !s.IsReady() {
		return BookListing{}, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return BookListing{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, layers, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return BookListing{}, util.Errorf("%w", err)
	}

	return BookListing{Book: book, Layers: layers}, nil
}

// drawUnusedBookID returns a random book ID that this shelf does not already
// hold, in its books or its trash. The exclusive shelf lock must be held.
//
// The ID is drawn at random, not derived from the layers and title: what keeps
// two books apart is the entropy behind it, not the probe here. The probe only
// sees books this process already knows about - the cache does not notice a book
// another machine added to a shared shelf, or one copied in with a file manager,
// until it rescans - so it is insurance against an ID this shelf demonstrably
// holds, and is expected never to fire.
func (s *Shelf) drawUnusedBookID() (string, error) {
	for range MaxBookIDCreationAttempts {
		candidate, idErr := newBookID()
		if idErr != nil {
			return "", util.Errorf("%w", idErr)
		}

		_, _, err := s.getUpdatedBookFromBookID(candidate)
		if errors.Is(err, ErrBookNotFound) {
			inTrash, trashErr := s.isBookIDInTrash(candidate)
			if trashErr != nil {
				return "", util.Errorf("%w", trashErr)
			}
			if !inTrash {
				return candidate, nil
			}
		} else if err != nil {
			return "", util.Errorf("%w", err)
		}
	}

	return "", util.NewError("failed to draw an unused book ID after multiple attempts")
}

// DeleteBook moves a book into trash by its ID.
func (s *Shelf) DeleteBook(bookID string) error {
	return s.MoveBookToTrash(bookID)
}

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

	for _, listing := range s.listBookListingsFromCache() {
		if listing.Layers.Equal(layers) {
			books = append(books, listing.Book)
		}
	}

	return books, nil
}

// MoveBook moves a book to new layers and returns the updated Book instance.
func (s *Shelf) MoveBook(bookID string, newLayers Layers) (*Book, error) {
	if err := validateLayers(newLayers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, _, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newLayerPath := path.Join(booksFolder, path.Join(newLayers...))
	err = root.MkdirAll(newLayerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.addLayersToBookCache(newLayers)

	newBookPath := path.Join(newLayerPath, path.Base(book.FolderPath()))
	err = root.Rename(book.FolderPath(), newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook, err := bookpkg.Open(s.dbRoot, s.Logger, newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.updateBookCacheEntry(newLayers, newBookPath, movedBook)

	return movedBook, nil
}

// CopyBook copies an existing book into targetLayer and returns the copy, which
// carries a fresh book ID so that the original and the copy can coexist in the
// same shelf.
//
// A book cannot be copied by hand with a file manager: the book cache keys on
// book.json's id, and two books that share an ID silently collapse to one in
// every listing. So the copy is staged in the app temp folder, given a new ID
// there, and only then published under books/ - the same transactional shape as
// NewBookWith, so a failure partway through leaves nothing behind but temp data
// that the next startup wipes. The whole package is reproduced verbatim, so the
// copy is self-contained: its sources, assets, cover and current-source pointer
// all come along, and the relative asset paths inside a source resolve without
// rewriting.
func (s *Shelf) CopyBook(bookID string, targetLayer Layers) (*Book, error) {
	if err := validateLayers(targetLayer); err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Refuse a read-only shelf before taking the lock; publishBookCopy asks again
	// once staging begins, so this only moves the refusal earlier.
	if _, err := s.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	sourceBook, _, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Refuse to copy a book this build cannot rewrite before touching the disk:
	// giving the copy a new ID rewrites book.json, and a book.json this build
	// does not understand would be downgraded in the process.
	if err := sourceBook.EnsureWritable(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Same shelf, so the staging root and the source root are one and the same;
	// the cross-shelf transfer in shelf_cross_book.go is the general case of this
	// same publish core.
	newBook, err := s.publishBookCopy(s.dbRoot, sourceBook, targetLayer, false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return newBook, nil
}

// bookFolderLayers derives a book's layer from where its package folder sits
// under booksFolder. The tree is the source of truth for a book's position, so
// this is what a scan uses to fill each cache entry's layers; every other read
// then takes the layer from that entry rather than recomputing it here.
//
// folderPath is always built with path.Join, which uses "/" on every platform,
// so it is split on "/" rather than os.PathSeparator. A book directly under
// booksFolder yields an empty (non-nil) Layers.
func bookFolderLayers(folderPath string) Layers {
	return strings.Split(path.Dir(folderPath), "/")[1:]
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

	var dfsFunc func(string, *dirChild, bool)

	dfsFunc = func(pth string, child *dirChild, trusted bool) {
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
		if shelfutil.IsIgnoredDir(folderName) {
			return
		}

		// Paths are always built with path.Join, which uses "/" on every
		// platform, so split on "/" rather than os.PathSeparator (which would
		// be "\" on Windows and break layer parsing).
		if strings.HasSuffix(folderName, bookExtension) {
			if onBook == nil {
				return
			}

			book, err := bookpkg.Open(s.dbRoot, s.Logger, pth)
			if err != nil {
				s.Error("Error opening book", "path", pth, "error", err)
				return
			}

			if !onBook(book) {
				skipAll = true
			}
			return
		}

		if onLayer != nil && !onLayer(strings.Split(pth, "/")[1:]) {
			skipAll = true
			return
		}

		children, childrenTrusted, err := dirCache.readDir(pth, trusted)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to read directory during book scan", "path", pth, "error", err)
			}
			return
		}

		for i := range children {
			fullPath := path.Join(pth, children[i].Name)
			dfsFunc(fullPath, &children[i], childrenTrusted)
		}
	}

	dfsFunc(booksFolder, nil, true)
	return dirCache.install(!skipAll), nil
}
