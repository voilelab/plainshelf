package main

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

func TestReaderShelfIDMatchesSyncNamespace(t *testing.T) {
	if readerapi.ShelfID != readingprogress.ReaderShelfID {
		t.Fatalf("readerapi.ShelfID (%q) must equal readingprogress.ReaderShelfID (%q)",
			readerapi.ShelfID, readingprogress.ReaderShelfID)
	}
}
