package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readhistory"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/readingstats"
	"github.com/voilelab/plainshelf/shelf"
)

type readerProjectionFixture struct {
	app      *DesktopApp
	unique   string
	multiple string
	missing  string
}

func newReaderProjectionFixture(t *testing.T) readerProjectionFixture {
	t.Helper()
	alphaRoot, betaRoot := t.TempDir(), t.TempDir()
	desktopApp := startedDesktopAppWithConfs(t, []*shelf.ShelfConfWithID{
		{ID: "alpha", ShelfConf: shelf.ShelfConf{LibRoot: alphaRoot}},
		{ID: "beta", ShelfConf: shelf.ShelfConf{LibRoot: betaRoot}},
	})
	alpha := mustShelf(t, desktopApp, "alpha")
	beta := mustShelf(t, desktopApp, "beta")

	unique, err := alpha.NewBook(nil, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	shared, err := alpha.NewBook(nil, "Shared")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	copyBookPackage(t, alphaRoot, betaRoot, "shared.bookpkg")
	if _, err := beta.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if _, err := beta.GetBook(shared.ID()); err != nil {
		t.Fatalf("shared book must be on both shelves: %v", err)
	}

	dir := t.TempDir()
	desktopApp.readHistoryPath = filepath.Join(dir, "read_history.json")
	desktopApp.readingStatsPath = filepath.Join(dir, "reading_stats.json")
	return readerProjectionFixture{app: desktopApp, unique: unique.ID(), multiple: shared.ID(), missing: "gone-book"}
}

func mustShelf(t *testing.T, desktopApp *DesktopApp, id string) *shelf.ShelfData {
	t.Helper()
	data, ok := desktopApp.app.ShelfManager().GetShelf(id)
	if !ok {
		t.Fatalf("shelf %s not found", id)
	}
	return data
}

func copyBookPackage(t *testing.T, fromRoot, toRoot, name string) {
	t.Helper()
	src := filepath.Join(fromRoot, "books", name)
	if err := os.CopyFS(filepath.Join(toRoot, "books", name), os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
}

func TestReadHistoryProjectsReaderBooks(t *testing.T) {
	f := newReaderProjectionFixture(t)
	store := readhistory.NewStore(f.app.readHistoryPath)
	for _, op := range []struct{ shelf, book string }{
		{"alpha", "older"},
		{readingprogress.ReaderShelfID, f.missing},
		{readingprogress.ReaderShelfID, f.multiple},
		{readingprogress.ReaderShelfID, f.unique},
	} {
		if err := store.Record(op.shelf, op.book); err != nil {
			t.Fatal(err)
		}
	}

	text, err := f.app.ReadReadHistory()
	if err != nil {
		t.Fatalf("ReadReadHistory: %v", err)
	}

	got := readhistory.Parse(text)
	if want := []string{f.unique, "older"}; !slices.Equal(got.Shelves["alpha"], want) {
		t.Errorf("alpha history = %v, want %v", got.Shelves["alpha"], want)
	}
	if want := []string{f.multiple, f.missing}; !slices.Equal(got.Shelves[readingprogress.ReaderShelfID], want) {
		t.Errorf("reader history = %v, want %v", got.Shelves[readingprogress.ReaderShelfID], want)
	}
	if _, onBeta := got.Shelves["beta"]; onBeta {
		t.Errorf("a book on two shelves was guessed onto beta: %v", got.Shelves["beta"])
	}
	if disk := mustReadFile(t, f.app.readHistoryPath); disk != text {
		t.Errorf("projection not persisted:\n disk %s\n read %s", disk, text)
	}
}

func TestReadingStatsProjectsReaderBooks(t *testing.T) {
	f := newReaderProjectionFixture(t)
	store := readingstats.NewStore(f.app.readingStatsPath)
	today := time.Now().Format(time.DateOnly)
	now := time.Now
	adds := []struct {
		key     string
		seconds float64
	}{
		{"alpha", 40},
		{readingstats.ReaderBookKey(readingprogress.ReaderShelfID, f.unique), 30},
		{readingstats.ReaderBookKey(readingprogress.ReaderShelfID, f.multiple), 20},
		{readingstats.ReaderBookKey(readingprogress.ReaderShelfID, f.missing), 10},
	}
	for _, add := range adds {
		if err := store.Add(add.key, today, add.seconds, now); err != nil {
			t.Fatal(err)
		}
	}

	text, err := f.app.ReadReadingStats()
	if err != nil {
		t.Fatalf("ReadReadingStats: %v", err)
	}

	got := readingstats.Parse(text)
	if v := got.Shelves["alpha"][today]; v != 70 {
		t.Errorf("alpha seconds = %d, want 70", v)
	}
	if _, pending := got.Shelves[readingstats.ReaderBookKey(readingprogress.ReaderShelfID, f.unique)]; pending {
		t.Error("projected reader seconds must leave the pending key")
	}
	for _, id := range []string{f.multiple, f.missing} {
		if v := got.Shelves[readingstats.ReaderBookKey(readingprogress.ReaderShelfID, id)][today]; v == 0 {
			t.Errorf("unplaceable book %s lost its pending seconds", id)
		}
	}
	if _, onBeta := got.Shelves["beta"]; onBeta {
		t.Error("a book on two shelves was guessed onto beta")
	}
	if disk := mustReadFile(t, f.app.readingStatsPath); disk != text {
		t.Errorf("projection not persisted:\n disk %s\n read %s", disk, text)
	}
}
