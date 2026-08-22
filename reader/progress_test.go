package main

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

// A desktop-launched reader now writes progress under the real shelf id passed
// via -shelf, so no projection is involved for that path. A standalone reader
// still falls back to readerapi.ShelfID, and the desktop side folds exactly the
// readingprogress.ReaderShelfID namespace onto real shelves. That fallback is the
// only remaining projection path, so its two ends must stay equal: if they drift,
// standalone reader progress would be written under a namespace the desktop never
// projects. (A real shelf can never take this id — see generateDesktopShelfID.)
func TestReaderShelfIDMatchesSyncNamespace(t *testing.T) {
	if readerapi.ShelfID != readingprogress.ReaderShelfID {
		t.Fatalf("readerapi.ShelfID (%q) must equal readingprogress.ReaderShelfID (%q)",
			readerapi.ShelfID, readingprogress.ReaderShelfID)
	}
}
