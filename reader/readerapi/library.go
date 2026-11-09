package readerapi

import (
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const ShelfID = "book"

type Library struct {
	mu     sync.RWMutex
	logger *logutil.Logger
	pkgs   map[string]*heldBook
}

type heldBook struct {
	pkg  *bookpkg.BookPackage
	refs int
}

func NewLibrary(logger *logutil.Logger) *Library {
	return &Library{logger: logger, pkgs: map[string]*heldBook{}}
}

func (l *Library) Open(dir string) (string, error) {
	pkg, err := bookpkg.OpenBookPackage(dir, l.logger)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	bookID := pkg.Book().ID()

	l.mu.Lock()
	defer l.mu.Unlock()

	if held := l.pkgs[bookID]; held != nil {
		held.refs++
		if err := pkg.Close(); err != nil {
			l.warn("failed to close a duplicate book package", "book_id", bookID, "error", err)
		}
		return bookID, nil
	}
	l.pkgs[bookID] = &heldBook{pkg: pkg, refs: 1}
	return bookID, nil
}

func (l *Library) CloseBook(bookID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	held := l.pkgs[bookID]
	if held == nil {
		return nil
	}
	if held.refs--; held.refs > 0 {
		return nil
	}
	delete(l.pkgs, bookID)
	return l.closePkg(held.pkg)
}

func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var first error
	for bookID, held := range l.pkgs {
		delete(l.pkgs, bookID)
		if err := l.closePkg(held.pkg); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (l *Library) closePkg(pkg *bookpkg.BookPackage) error {
	if pkg == nil {
		return nil
	}
	if err := pkg.Close(); err != nil {
		l.warn("failed to close a book package", "book_id", pkg.Book().ID(), "error", err)
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *Library) Has(bookID string) bool {
	return l.withBook(bookID, func(*bookpkg.Book) {})
}

func (l *Library) withBook(bookID string, read func(*bookpkg.Book)) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	held := l.pkgs[bookID]
	if held == nil {
		return false
	}

	read(held.pkg.Book())
	return true
}

func (l *Library) warn(msg string, args ...any) {
	if l.logger != nil {
		l.logger.Warn(msg, args...)
	}
}
