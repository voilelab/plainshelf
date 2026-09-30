package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

// The full close path the frontend drives: the reader stages its latest position
// as the user scrolls, the window closes (beforeClose), and reopening reads that
// exact position back — no 10s autosave involved.
func TestReaderStageThenCloseRoundTrips(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	app := &ReaderApp{
		shelfID:        readerapi.ShelfID,
		progressStore:  store,
		progressStager: readingclose.NewStager(store, time.Second),
	}

	// The frontend stages on every scroll; take the last one before closing.
	app.StageReadingProgress(readerapi.ShelfID, "book-a", 4321, 1000)
	if prevent := app.beforeClose(t.Context()); prevent {
		t.Fatal("beforeClose must allow the close")
	}

	// Reopening reads the reader's own namespace back.
	text, err := app.ReadReadingProgress()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := readingprogress.Parse(text)
	if v := got.Shelves[readerapi.ShelfID]["book-a"].Offset; v != 4321 {
		t.Fatalf("restored offset = %d, want 4321 (staged position must survive the close)", v)
	}
}
