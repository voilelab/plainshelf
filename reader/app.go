package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
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
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails v3.1.0 sets this header on every asset request (application.webViewAssetRequest).
const windowIDHeader = "x-wails-window-id"

type readerWindow struct {
	shelfID    string
	standalone bool
	section    int
	bookID     string
	prompted   bool
	stager     *readingclose.Stager
}

type ReaderApp struct {
	native  nativeUI
	library *readerapi.Library

	progressStore *readingprogress.Store
	historyStore  *readhistory.Store
	statsStore    *readingstats.Store
	preferences   *preferences.Store
	closeTimeout  time.Duration

	openMu sync.Mutex

	mu      sync.Mutex
	windows map[uint]*readerWindow

	closeOnce sync.Once
}

func NewReaderApp(native nativeUI, logger *logutil.Logger) *ReaderApp {
	app := &ReaderApp{
		native:       native,
		library:      readerapi.NewLibrary(logger),
		closeTimeout: readingclose.DefaultTimeout,
		windows:      map[uint]*readerWindow{},
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

	return app
}

func (a *ReaderApp) newWindowState(shelfID, bookID string, section int) *readerWindow {
	shelfID = strings.TrimSpace(shelfID)
	state := &readerWindow{
		shelfID:    shelfID,
		standalone: shelfID == "",
		section:    section,
		bookID:     bookID,
		stager:     readingclose.NewStager(a.progressStore, a.closeTimeout),
	}
	if state.standalone {
		state.shelfID = readerapi.ShelfID
	}
	return state
}

func (a *ReaderApp) openWindow(state *readerWindow) uint {
	return a.native.newWindow(func(win uint) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.windows[win] = state
	})
}

func (a *ReaderApp) Launch(launch launchArgs) uint {
	bookID := ""
	if launch.bookPath != "" {
		id, err := a.library.Open(launch.bookPath)
		if err != nil {
			log.Println("failed to open the book package given on the command line:", err)
		} else {
			bookID = id
		}
	}
	return a.openWindow(a.newWindowState(launch.shelfID, bookID, launch.section))
}

func (a *ReaderApp) window(win uint) (readerWindow, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	state, ok := a.windows[win]
	if !ok {
		return readerWindow{shelfID: readerapi.ShelfID, standalone: true, section: noLaunchSection}, false
	}
	return *state, true
}

func (a *ReaderApp) windowShowing(bookID string) (uint, *readerWindow) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for win, state := range a.windows {
		if state.bookID == bookID {
			return win, state
		}
	}
	return 0, nil
}

func windowOf(ctx context.Context) uint {
	if ctx == nil {
		return 0
	}
	if win, ok := ctx.Value(application.WindowKey).(interface{ ID() uint }); ok {
		return win.ID()
	}
	return 0
}

func (a *ReaderApp) windowClosed(win uint) {
	a.mu.Lock()
	state := a.windows[win]
	delete(a.windows, win)
	a.mu.Unlock()

	if state != nil {
		a.closeWindowBook(state)
	}
}

func (a *ReaderApp) closeWindowBook(state *readerWindow) {
	state.stager.PersistOnClose()
	if state.bookID == "" {
		return
	}
	if err := a.library.CloseBook(state.bookID); err != nil {
		log.Println("failed to close a book package:", err)
	}
}

func (a *ReaderApp) Shutdown(context.Context) {
	a.closeOnce.Do(a.closeInOrder)
}

func (a *ReaderApp) closeInOrder() {
	a.mu.Lock()
	states := make([]*readerWindow, 0, len(a.windows))
	for win, state := range a.windows {
		states = append(states, state)
		delete(a.windows, win)
	}
	a.mu.Unlock()

	var wg sync.WaitGroup
	for _, state := range states {
		wg.Go(func() { a.closeWindowBook(state) })
	}
	wg.Wait()

	if err := a.library.Close(); err != nil {
		log.Println("failed to close the open book packages:", err)
	}
}

func (a *ReaderApp) StageReadingProgress(shelfID, bookID string, offset, at int64) {
	if _, state := a.windowShowing(bookID); state != nil {
		state.stager.Stage(shelfID, bookID, offset, at)
	}
}

func (a *ReaderApp) windowReady(win uint) {
	a.openMu.Lock()
	a.mu.Lock()
	state := a.windows[win]
	prompt := state != nil && state.bookID == "" && !state.prompted
	if prompt {
		state.prompted = true
	}
	a.mu.Unlock()
	a.openMu.Unlock()

	if !prompt {
		return
	}
	if _, err := a.openBookPackageIn(win); err != nil {
		log.Println("failed to open a book package:", err)
	}
}

func (a *ReaderApp) OpenBookPackage(ctx context.Context) (string, error) {
	return a.openBookPackageIn(windowOf(ctx))
}

func (a *ReaderApp) openBookPackageIn(win uint) (string, error) {
	dir, err := a.native.openDirectory(win, "Open a book")
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if dir == "" {
		return "", nil
	}

	bookID, err := a.openBook(win, dir, "", noLaunchSection)
	if err != nil {
		a.showNotABook(win)
		return "", util.Errorf("%w", err)
	}
	return bookID, nil
}

func (a *ReaderApp) showNotABook(win uint) {
	err := a.native.showError(win, "Cannot open this folder",
		"That folder is not a book package: it has no readable book.json.")
	if err != nil {
		log.Println("failed to show book-open error dialog:", err)
	}
}

func (a *ReaderApp) openBook(origin uint, dir, shelfID string, section int) (string, error) {
	a.openMu.Lock()
	defer a.openMu.Unlock()

	if err := checkBookPackageDir(dir); err != nil {
		return "", err
	}
	bookID, err := a.library.Open(dir)
	if err != nil {
		return "", err
	}

	if win, _ := a.windowShowing(bookID); win != 0 {
		if err := a.library.CloseBook(bookID); err != nil {
			log.Println("failed to release a reopened book package:", err)
		}
		a.native.focus(win)
		return bookID, nil
	}

	next := a.newWindowState(shelfID, bookID, section)
	if win := a.fillEmptyWindow(origin, next); win != 0 {
		a.native.reload(win)
		a.native.focus(win)
		return bookID, nil
	}
	a.native.focus(a.openWindow(next))
	return bookID, nil
}

func (a *ReaderApp) fillEmptyWindow(origin uint, next *readerWindow) uint {
	a.mu.Lock()
	defer a.mu.Unlock()

	target := uint(0)
	if state, ok := a.windows[origin]; ok && state.bookID == "" {
		target = origin
	} else {
		for win, state := range a.windows {
			if state.bookID == "" {
				target = win
				break
			}
		}
	}
	if target == 0 {
		return 0
	}
	next.prompted = a.windows[target].prompted
	a.windows[target] = next
	return target
}

func (a *ReaderApp) BootFor(r *http.Request) readerapi.BootConfig {
	win, err := strconv.ParseUint(r.Header.Get(windowIDHeader), 10, strconv.IntSize)
	if err != nil {
		win = 0
	}
	return a.bootConfig(uint(win))
}

func (a *ReaderApp) bootConfig(win uint) readerapi.BootConfig {
	state, _ := a.window(win)
	boot := readerapi.BootConfig{ShelfID: state.shelfID, BookID: state.bookID}
	if state.section >= 0 {
		section := state.section
		boot.Section = &section
	}
	return boot
}

func (a *ReaderApp) Library() *readerapi.Library {
	return a.library
}

func (a *ReaderApp) ReadReadingProgress(ctx context.Context) (string, error) {
	return a.readProgressIn(windowOf(ctx))
}

func (a *ReaderApp) readProgressIn(win uint) (string, error) {
	if a.progressStore == nil {
		return "", nil
	}
	state, _ := a.window(win)
	doc, _, err := a.progressStore.Read()
	if err != nil {
		return "", err
	}
	books := doc.Shelves[state.shelfID]
	if len(books) == 0 {
		return "", nil
	}
	filtered := readingprogress.New()
	filtered.Shelves[state.shelfID] = books
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

func (a *ReaderApp) AddReadingSeconds(ctx context.Context, key, date string, seconds float64) error {
	return a.addSecondsIn(windowOf(ctx), key, date, seconds)
}

func (a *ReaderApp) addSecondsIn(win uint, key, date string, seconds float64) error {
	return a.statsStore.Add(a.readingStatsKey(win, key), date, seconds, time.Now)
}

func (a *ReaderApp) readingStatsKey(win uint, key string) string {
	state, _ := a.window(win)
	if !state.standalone || key != readerapi.ShelfID || state.bookID == "" {
		return key
	}
	return readingstats.ReaderBookKey(key, state.bookID)
}
