// Package readerapi serves one open book package over the same read-only HTTP
// surface the PlainShelf server exposes, so the shared frontend can talk to the
// reader app without knowing there is no server behind it.
//
// Everything here is deliberately free of Wails imports: the window is one
// caller of this handler, and a test is another.
package readerapi

import (
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// ShelfID is the shelf the reader reports itself as holding.
//
// The frontend addresses books as /api/shelves/{shelf}/books/{book} whatever it
// is talking to, so a single-book reader still needs one shelf id. It is a
// constant rather than derived from the folder: nothing in the app looks a
// shelf up by it.
const ShelfID = "book"

// Library is the one book the window currently shows, if any.
//
// The app opens a new package when the user picks another folder, so reads and
// the swap have to be serialized: an in-flight request must not see a package
// that has just been closed underneath it.
type Library struct {
	mu     sync.RWMutex
	logger *logutil.Logger
	pkg    *bookpkg.BookPackage
}

func NewLibrary(logger *logutil.Logger) *Library {
	return &Library{logger: logger}
}

// Open reads the book package in dir and makes it the open book, closing
// whatever was open before. It returns the book's ID.
//
// A failed open leaves the previously open book in place: a user who picks the
// wrong folder should get an error, not an app with nothing in it.
//
// The swap waits for every in-flight read to finish and closes the previous
// package while holding the write lock. Closing it earlier would pull the
// directory handle out from under a request that has already resolved the book
// but not yet opened its source, which surfaces as a truncated response rather
// than as anything the user could act on.
func (l *Library) Open(dir string) (string, error) {
	pkg, err := bookpkg.OpenBookPackage(dir, l.logger)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// A failed close is logged by closeLocked and goes no further: the book the
	// user just picked still becomes the open one.
	_ = l.closeLocked()
	l.pkg = pkg

	return pkg.Book().ID(), nil
}

// Close releases the open package, if there is one. Like Open, it waits for
// in-flight reads rather than closing underneath them.
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.closeLocked()
}

// closeLocked closes the open package. The caller holds the write lock, so no
// read can be in progress through the handle being closed.
func (l *Library) closeLocked() error {
	pkg := l.pkg
	l.pkg = nil

	if pkg == nil {
		return nil
	}
	if err := pkg.Close(); err != nil {
		if l.logger != nil {
			l.logger.Warn("failed to close a book package", "error", err)
		}
		return util.Errorf("%w", err)
	}
	return nil
}

// BookID is the open book's ID, or "" when no book is open.
func (l *Library) BookID() string {
	bookID := ""
	l.withBook(func(book *bookpkg.Book) { bookID = book.ID() })
	return bookID
}

// withBook runs read with the open book, holding the package open for the whole
// call. It reports whether there was a book to run against.
//
// The lock is held across the request rather than only while the book is looked
// up: a handler resolves the book, then opens a source or an asset through the
// same directory handle, and those are separate reads. Holding a package that
// is already open costs a reader lock on a single-window app; not holding it
// costs a request that fails halfway through its body.
func (l *Library) withBook(read func(*bookpkg.Book)) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pkg == nil {
		return false
	}

	read(l.pkg.Book())
	return true
}
