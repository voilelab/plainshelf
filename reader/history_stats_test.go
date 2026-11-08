package main

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readhistory"
	"github.com/voilelab/plainshelf/internal/readingstats"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

func withDeviceStores(t *testing.T, app *ReaderApp) *ReaderApp {
	t.Helper()
	dir := t.TempDir()
	app.historyStore = readhistory.NewStore(filepath.Join(dir, "read_history.json"))
	app.statsStore = readingstats.NewStore(filepath.Join(dir, "reading_stats.json"))
	return app
}

func readStats(t *testing.T, app *ReaderApp) readingstats.Document {
	t.Helper()
	text, err := app.ReadReadingStats()
	if err != nil {
		t.Fatalf("ReadReadingStats: %v", err)
	}
	return readingstats.Parse(text)
}

func TestStandaloneStatsKeyedByBook(t *testing.T) {
	app := newStatsApp(t)
	win := launchBook(t, app, "")
	today := time.Now().Format(time.DateOnly)

	if err := app.AddReadingSeconds(windowContext(t, win), readerapi.ShelfID, today, 30); err != nil {
		t.Fatalf("AddReadingSeconds: %v", err)
	}

	doc := readStats(t, app)
	if v := doc.Shelves[readingstats.ReaderBookKey(readerapi.ShelfID, testBookID)][today]; v != 30 {
		t.Errorf("book-keyed seconds = %d, want 30: %v", v, doc.Shelves)
	}
	if _, ok := doc.Shelves[readerapi.ShelfID]; ok {
		t.Errorf("seconds written under the bare reader shelf: %v", doc.Shelves)
	}
}

func TestRealShelfStatsKeyedByShelf(t *testing.T) {
	app := newStatsApp(t)
	win := launchBook(t, app, "shelf-real")
	today := time.Now().Format(time.DateOnly)

	if err := app.addSecondsIn(win, "shelf-real", today, 30); err != nil {
		t.Fatalf("AddReadingSeconds: %v", err)
	}

	if v := readStats(t, app).Shelves["shelf-real"][today]; v != 30 {
		t.Errorf("shelf-real seconds = %d, want 30", v)
	}
}

func TestRealBookShelfStatsKeyedByShelf(t *testing.T) {
	app := newStatsApp(t)
	win := launchBook(t, app, readerapi.ShelfID)
	today := time.Now().Format(time.DateOnly)

	if err := app.addSecondsIn(win, readerapi.ShelfID, today, 30); err != nil {
		t.Fatalf("AddReadingSeconds: %v", err)
	}

	if v := readStats(t, app).Shelves[readerapi.ShelfID][today]; v != 30 {
		t.Errorf("seconds for a real %q shelf = %d, want 30", readerapi.ShelfID, v)
	}
}

func TestStandaloneStatsPerWindowBook(t *testing.T) {
	app := newStatsApp(t)
	first := launchBook(t, app, "")
	second := launchBookAt(t, app, "", "otherbook", noLaunchSection)
	today := time.Now().Format(time.DateOnly)

	for win, seconds := range map[uint]float64{first: 10, second: 20} {
		if err := app.addSecondsIn(win, readerapi.ShelfID, today, seconds); err != nil {
			t.Fatal(err)
		}
	}

	doc := readStats(t, app)
	for bookID, want := range map[string]int64{testBookID: 10, "otherbook": 20} {
		if v := doc.Shelves[readingstats.ReaderBookKey(readerapi.ShelfID, bookID)][today]; v != want {
			t.Errorf("%s seconds = %d, want %d: %v", bookID, v, want, doc.Shelves)
		}
	}
}

func newStatsApp(t *testing.T) *ReaderApp {
	t.Helper()
	app, _ := newTestApp(t)
	return withDeviceStores(t, app)
}

func TestReaderRecordsReadHistory(t *testing.T) {
	app := newStatsApp(t)

	if err := app.RecordReadHistory("shelf-real", testBookID); err != nil {
		t.Fatalf("RecordReadHistory: %v", err)
	}
	text, err := app.ReadReadHistory()
	if err != nil {
		t.Fatalf("ReadReadHistory: %v", err)
	}
	if got := readhistory.Parse(text).Shelves["shelf-real"]; !slices.Equal(got, []string{testBookID}) {
		t.Errorf("history = %v, want [%s]", got, testBookID)
	}
}

func TestReaderUsesDesktopDocumentPaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("path isolation uses XDG_CONFIG_HOME")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := NewReaderApp(nil, nil)
	t.Cleanup(func() { app.Shutdown(t.Context()) })
	today := time.Now().Format(time.DateOnly)

	if err := app.RecordReadHistory("shelf-real", testBookID); err != nil {
		t.Fatal(err)
	}
	if err := app.addSecondsIn(0, "shelf-real", today, 30); err != nil {
		t.Fatal(err)
	}

	historyPath, err := readhistory.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	statsPath, err := readingstats.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := readhistory.NewStore(historyPath).Read(); len(readhistory.Parse(text).Shelves["shelf-real"]) != 1 {
		t.Errorf("history not at the shared path %s: %q", historyPath, text)
	}
	if text, _ := readingstats.NewStore(statsPath).Read(); readingstats.Parse(text).Shelves["shelf-real"][today] != 30 {
		t.Errorf("stats not at the shared path %s: %q", statsPath, text)
	}
}
