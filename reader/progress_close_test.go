package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

func TestReaderStageThenCloseRoundTrips(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	app := &ReaderApp{
		shelfID:        readerapi.ShelfID,
		progressStore:  store,
		progressStager: readingclose.NewStager(store, time.Second),
	}

	app.StageReadingProgress(readerapi.ShelfID, "book-a", 4321, 1000)
	if prevent := app.beforeClose(t.Context()); prevent {
		t.Fatal("beforeClose must allow the close")
	}

	text, err := app.ReadReadingProgress()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := readingprogress.Parse(text)
	if v := got.Shelves[readerapi.ShelfID]["book-a"].Offset; v != 4321 {
		t.Fatalf("restored offset = %d, want 4321 (staged position must survive the close)", v)
	}
}
