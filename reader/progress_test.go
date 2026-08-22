package main

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

// The reader writes progress under readerapi.ShelfID, and the desktop projection
// folds exactly the readingprogress.ReaderShelfID namespace. If these ever drift,
// reader progress would be written under a namespace the desktop never projects.
func TestReaderShelfIDMatchesSyncNamespace(t *testing.T) {
	if readerapi.ShelfID != readingprogress.ReaderShelfID {
		t.Fatalf("readerapi.ShelfID (%q) must equal readingprogress.ReaderShelfID (%q)",
			readerapi.ShelfID, readingprogress.ReaderShelfID)
	}
}
