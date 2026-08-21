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
	"github.com/voilelab/plainshelf/shelf"
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
	pkg    *shelf.BookPackage
}

func NewLibrary(logger *logutil.Logger) *Library {
	return &Library{logger: logger}
}

// Open reads the book package in dir and makes it the open book, closing
// whatever was open before. It returns the book's ID.
//
// A failed open leaves the previously open book in place: a user who picks the
// wrong folder should get an error, not an app with nothing in it.
func (l *Library) Open(dir string) (string, error) {
	pkg, err := shelf.OpenBookPackage(dir, l.logger)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	l.mu.Lock()
	previous := l.pkg
	l.pkg = pkg
	l.mu.Unlock()

	if previous != nil {
		if err := previous.Close(); err != nil && l.logger != nil {
			l.logger.Warn("failed to close the previously open book package", "error", err)
		}
	}

	return pkg.Book().ID(), nil
}

// Close releases the open package, if there is one.
func (l *Library) Close() error {
	l.mu.Lock()
	pkg := l.pkg
	l.pkg = nil
	l.mu.Unlock()

	if pkg == nil {
		return nil
	}
	if err := pkg.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// BookID is the open book's ID, or "" when no book is open.
func (l *Library) BookID() string {
	book, ok := l.book()
	if !ok {
		return ""
	}
	return book.ID()
}

func (l *Library) book() (*shelf.Book, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pkg == nil {
		return nil, false
	}
	return l.pkg.Book(), true
}
