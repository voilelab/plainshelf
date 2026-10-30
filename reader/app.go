package main

import (
	"context"
	"log"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/reader/readerapi"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ReaderApp struct {
	ctx     context.Context
	library *readerapi.Library

	shelfID string

	launchSection int

	progressStore *readingprogress.Store

	promptedForBook bool

	progressStager *readingclose.Stager
}

func NewReaderApp(logger *logutil.Logger, shelfID string, launchSection int) *ReaderApp {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		shelfID = readerapi.ShelfID
	}

	app := &ReaderApp{library: readerapi.NewLibrary(logger), shelfID: shelfID, launchSection: launchSection}

	if path, err := readingprogress.DefaultProgressPath(); err != nil {
		log.Println("reading progress will not be persisted:", err)
	} else {
		app.progressStore = readingprogress.NewStore(path)
	}

	app.progressStager = readingclose.NewStager(app.progressStore, readingclose.DefaultTimeout)

	return app
}

func (a *ReaderApp) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *ReaderApp) beforeClose(context.Context) (prevent bool) {
	a.progressStager.PersistOnClose()
	return false
}

func (a *ReaderApp) StageReadingProgress(shelfID, bookID string, offset, at int64) {
	a.progressStager.Stage(shelfID, bookID, offset, at)
}

// Wails v2.16.0 WailsContext.m presents macOS directory dialogs with
// beginSheetModalForWindow:self.mainWindow, so the window must exist first.
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

	bookID, err := a.openStandaloneBook(dir)
	if err != nil {
		_, dialogErr := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
			Type:    wailsruntime.ErrorDialog,
			Title:   "Cannot open this folder",
			Message: "That folder is not a book package: it has no readable book.json.",
		})
		if dialogErr != nil {
			log.Println("failed to show book-open error dialog:", dialogErr)
		}
		return "", util.Errorf("%w", err)
	}

	wailsruntime.WindowReloadApp(a.ctx)
	return bookID, nil
}

func (a *ReaderApp) openStandaloneBook(dir string) (string, error) {
	bookID, err := a.library.Open(dir)
	if err != nil {
		return "", err
	}
	a.shelfID = readerapi.ShelfID
	a.launchSection = noLaunchSection
	return bookID, nil
}

func (a *ReaderApp) BootConfig() readerapi.BootConfig {
	boot := readerapi.BootConfig{ShelfID: a.shelfID, BookID: a.library.BookID()}

	if a.launchSection >= 0 {
		section := a.launchSection
		boot.Section = &section
	}
	return boot
}

func (a *ReaderApp) Library() *readerapi.Library {
	return a.library
}

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

func (a *ReaderApp) WriteReadingProgress(doc string) error {
	if a.progressStore == nil {
		return util.NewError("reading progress storage is not ready")
	}
	incoming, err := readingprogress.ParseStrict(doc)
	if err != nil {
		return err
	}
	_, err = a.progressStore.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeNewest(disk, incoming)
	})
	return err
}
