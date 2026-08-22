package main

import (
	"context"
	"log"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/reader/readerapi"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ReaderApp is the window's side of the reader: it owns the open book package
// and the one native affordance this app has, which is asking the user for a
// folder.
//
// Reading itself goes through readerapi, not through bindings: that keeps the
// shared frontend on the provider it already uses for a server.
type ReaderApp struct {
	ctx     context.Context
	library *readerapi.Library

	// shelfID is the shelf id this reader reports as active and keys its progress
	// under. It is the real desktop shelf id when the desktop app launched the
	// reader with -shelf, so the reader opens at — and writes back to — the same
	// shelves.<realShelfID>.<bookID> position the desktop library already holds.
	// When the reader runs standalone (no -shelf) it is the synthetic
	// readerapi.ShelfID ("book"), which the desktop side projects onto real
	// shelves; see internal/readingprogress.
	shelfID string

	// progressStore is the shared reading-progress document, the same file the
	// desktop app keeps. It is nil only when the user config directory could not
	// be resolved, in which case progress simply is not persisted.
	progressStore *readingprogress.Store

	// promptedForBook records that the startup prompt already ran, so a reload
	// after opening a book does not reopen the dialog.
	promptedForBook bool
}

// NewReaderApp builds the reader window's app. shelfID is the active shelf id
// the reader reports and stores progress under; an empty value falls back to the
// synthetic readerapi.ShelfID, which is the standalone (no -shelf) case.
func NewReaderApp(logger *logutil.Logger, shelfID string) *ReaderApp {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		shelfID = readerapi.ShelfID
	}

	app := &ReaderApp{library: readerapi.NewLibrary(logger), shelfID: shelfID}

	// Persist progress to the desktop app's reading_progress.json so a book read
	// in the standalone reader shows the same position in the desktop library.
	// Both processes coordinate through the store's cross-process lock.
	if path, err := readingprogress.DefaultProgressPath(); err != nil {
		log.Println("reading progress will not be persisted:", err)
	} else {
		app.progressStore = readingprogress.NewStore(path)
	}

	return app
}

func (a *ReaderApp) Startup(ctx context.Context) {
	a.ctx = ctx
}

// DomReady asks for a book the first time the window has nothing to show.
//
// The prompt lives here rather than in Startup because a dialog needs a window
// to be modal to. A user who cancels keeps an empty window with the File menu,
// which is the only other way in.
func (a *ReaderApp) DomReady(context.Context) {
	if a.promptedForBook || a.library.BookID() != "" {
		return
	}
	a.promptedForBook = true

	if _, err := a.OpenBookPackage(); err != nil {
		log.Println("failed to open a book package:", err)
	}
}

func (a *ReaderApp) Shutdown(context.Context) {
	if err := a.library.Close(); err != nil {
		log.Println("failed to close the open book package:", err)
	}
}

// OpenBookPackage asks the user for a book folder and opens it.
//
// Bound to the frontend as well as called from the File menu: the shell's
// holding screen is where a user lands when they cancelled the first prompt.
// The window is reloaded rather than told about the new book, because the book
// reaches the frontend through index.html.
func (a *ReaderApp) OpenBookPackage() (string, error) {
	if a.ctx == nil {
		return "", nil
	}

	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open a book",
	})
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if dir == "" {
		return "", nil
	}

	bookID, err := a.library.Open(dir)
	if err != nil {
		wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{ //nolint:errcheck // the open error is what matters
			Type:    wailsruntime.ErrorDialog,
			Title:   "Cannot open this folder",
			Message: "That folder is not a book package: it has no readable book.json.",
		})
		return "", util.Errorf("%w", err)
	}

	wailsruntime.WindowReloadApp(a.ctx)
	return bookID, nil
}

// BootConfig is what the served index.html carries into the frontend. The
// frontend uses ShelfID as its active shelf, so injecting the real shelf id here
// is what makes a desktop-launched reader address the book — and read its stored
// progress — under the desktop library's own shelf.
func (a *ReaderApp) BootConfig() readerapi.BootConfig {
	return readerapi.BootConfig{ShelfID: a.shelfID, BookID: a.library.BookID()}
}

func (a *ReaderApp) Library() *readerapi.Library {
	return a.library
}

// ReadReadingProgress returns the reader's own progress from the shared
// document, or an empty string when it has stored none. Only the reader's active
// shelf namespace is returned: when launched from desktop that is the real shelf
// (so the reader opens at the desktop library's existing position); when
// standalone it is the synthetic readerapi.ShelfID. Other shelves' entries live
// in the same file but are not this reader's to display or rewrite.
//
// The frontend detects this binding (frontend/src/api/desktop.ts) and stores
// progress through the shared file rather than WebView localStorage.
func (a *ReaderApp) ReadReadingProgress() (string, error) {
	if a.progressStore == nil {
		return "", nil
	}
	doc, _, err := a.progressStore.Read()
	if err != nil {
		return "", err
	}
	books := doc.Shelves[a.shelfID]
	if len(books) == 0 {
		return "", nil
	}
	filtered := readingprogress.New()
	filtered.Shelves[a.shelfID] = books
	return readingprogress.Serialize(filtered)
}

// WriteReadingProgress records the reader's progress into the shared document.
// It only ever touches the one book it has open, under its active shelf
// namespace: every other entry in the same file — other shelves, and any other
// book — is re-read under the store's lock and preserved, so the reader can never
// clobber (or its stale in-memory copy roll back) progress another process owns.
// When launched from desktop the active shelf is the real shelf, so the write
// lands straight at shelves.<realShelfID>.<bookID> with no projection needed.
func (a *ReaderApp) WriteReadingProgress(doc string) error {
	if a.progressStore == nil {
		return util.NewError("reading progress storage is not ready")
	}
	incoming, err := readingprogress.ParseStrict(doc)
	if err != nil {
		return err
	}
	// The reader shows exactly one book, so only that book's entry is this
	// process's to write; merging at book granularity keeps a second reader
	// process's concurrent write to another book from being lost.
	bookID := a.library.BookID()
	_, err = a.progressStore.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeReaderBookWrite(disk, incoming, a.shelfID, bookID)
	})
	return err
}
