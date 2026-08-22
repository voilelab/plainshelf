package main

import (
	"context"
	"log"

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

	// progressStore is the shared reading-progress document, the same file the
	// desktop app keeps. It is nil only when the user config directory could not
	// be resolved, in which case progress simply is not persisted.
	progressStore *readingprogress.Store

	// promptedForBook records that the startup prompt already ran, so a reload
	// after opening a book does not reopen the dialog.
	promptedForBook bool
}

func NewReaderApp(logger *logutil.Logger) *ReaderApp {
	app := &ReaderApp{library: readerapi.NewLibrary(logger)}

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

// BootConfig is what the served index.html carries into the frontend.
func (a *ReaderApp) BootConfig() readerapi.BootConfig {
	return readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: a.library.BookID()}
}

func (a *ReaderApp) Library() *readerapi.Library {
	return a.library
}

// ReadReadingProgress returns the reader's own progress from the shared
// document, or an empty string when it has stored none. Only the reader's shelf
// namespace is returned: the desktop app's real-shelf entries live in the same
// file but are not the reader's to display or rewrite.
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
	books := doc.Shelves[readerapi.ShelfID]
	if len(books) == 0 {
		return "", nil
	}
	filtered := readingprogress.New()
	filtered.Shelves[readerapi.ShelfID] = books
	return readingprogress.Serialize(filtered)
}

// WriteReadingProgress records the reader's progress into the shared document.
// It only ever touches the reader's own shelf namespace: the desktop app's
// real-shelf entries in the same file are re-read under the store's lock and
// preserved, so the reader can never clobber (or its stale in-memory copy roll
// back) progress the desktop side has projected.
func (a *ReaderApp) WriteReadingProgress(doc string) error {
	if a.progressStore == nil {
		return util.NewError("reading progress storage is not ready")
	}
	incoming, err := readingprogress.ParseStrict(doc)
	if err != nil {
		return err
	}
	_, err = a.progressStore.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeReaderWrite(disk, incoming, readerapi.ShelfID)
	})
	return err
}
