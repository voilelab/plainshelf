package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/preferences"
	"github.com/voilelab/plainshelf/internal/readhistory"
	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/readingstats"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

type ReaderApp struct {
	native  nativeUI
	library *readerapi.Library

	shelfID    string
	standalone bool

	launchSection int

	progressStore *readingprogress.Store
	historyStore  *readhistory.Store
	statsStore    *readingstats.Store
	preferences   *preferences.Store

	promptedForBook bool

	progressStager *readingclose.Stager
	closeOnce      sync.Once
}

func NewReaderApp(native nativeUI, logger *logutil.Logger, shelfID string, launchSection int) *ReaderApp {
	shelfID = strings.TrimSpace(shelfID)
	standalone := shelfID == ""
	if standalone {
		shelfID = readerapi.ShelfID
	}

	app := &ReaderApp{
		native:        native,
		library:       readerapi.NewLibrary(logger),
		shelfID:       shelfID,
		standalone:    standalone,
		launchSection: launchSection,
	}

	if path, err := readingprogress.DefaultProgressPath(); err != nil {
		log.Println("reading progress will not be persisted:", err)
	} else {
		app.progressStore = readingprogress.NewStore(path)
	}

	historyPath, err := readhistory.DefaultPath()
	if err != nil {
		log.Println("read history will not be persisted:", err)
	}
	app.historyStore = readhistory.NewStore(historyPath)

	statsPath, err := readingstats.DefaultPath()
	if err != nil {
		log.Println("reading stats will not be persisted:", err)
	}
	app.statsStore = readingstats.NewStore(statsPath)

	preferencesPath, err := preferences.DefaultPath()
	if err != nil {
		log.Println("preferences will not be persisted:", err)
	}
	app.preferences = preferences.NewStore(preferencesPath, preferences.AppReader)

	app.progressStager = readingclose.NewStager(app.progressStore, readingclose.DefaultTimeout)

	return app
}

func (a *ReaderApp) beforeClose(ctx context.Context) (prevent bool) {
	a.Shutdown(ctx)
	return false
}

func (a *ReaderApp) StageReadingProgress(shelfID, bookID string, offset, at int64) {
	a.progressStager.Stage(shelfID, bookID, offset, at)
}

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
	a.closeOnce.Do(a.closeInOrder)
}

func (a *ReaderApp) closeInOrder() {
	a.progressStager.PersistOnClose()
	if err := a.library.Close(); err != nil {
		log.Println("failed to close the open book package:", err)
	}
}

func (a *ReaderApp) OpenBookPackage() (string, error) {
	dir, err := a.native.openDirectory("Open a book")
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if dir == "" {
		return "", nil
	}

	bookID, err := a.openStandaloneBook(dir)
	if err != nil {
		dialogErr := a.native.showError("Cannot open this folder",
			"That folder is not a book package: it has no readable book.json.")
		if dialogErr != nil {
			log.Println("failed to show book-open error dialog:", dialogErr)
		}
		return "", util.Errorf("%w", err)
	}

	a.native.reloadApp()
	return bookID, nil
}

func (a *ReaderApp) openStandaloneBook(dir string) (string, error) {
	bookID, err := a.library.Open(dir)
	if err != nil {
		return "", err
	}
	a.shelfID = readerapi.ShelfID
	a.standalone = true
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

func (a *ReaderApp) GetPreferences() (map[string]string, error) {
	return a.preferences.Read()
}

func (a *ReaderApp) SetPreference(key, value string) error {
	return a.preferences.Set(key, value)
}

func (a *ReaderApp) ReadReadHistory() (string, error) {
	return a.historyStore.Read()
}

func (a *ReaderApp) RecordReadHistory(shelfKey, bookID string) error {
	return a.historyStore.Record(shelfKey, bookID)
}

func (a *ReaderApp) ClearReadHistory(shelfKey string) error {
	return a.historyStore.Clear(shelfKey)
}

func (a *ReaderApp) SetReadHistoryLimit(limit int) error {
	return a.historyStore.SetLimit(limit)
}

func (a *ReaderApp) ReadReadingStats() (string, error) {
	return a.statsStore.Read()
}

func (a *ReaderApp) AddReadingSeconds(key, date string, seconds float64) error {
	return a.statsStore.Add(a.readingStatsKey(key), date, seconds, time.Now)
}

func (a *ReaderApp) readingStatsKey(key string) string {
	bookID := a.library.BookID()
	if !a.standalone || key != readerapi.ShelfID || bookID == "" {
		return key
	}
	return readingstats.ReaderBookKey(key, bookID)
}
