package readingclose

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingprogress"
)

func offsetOf(t *testing.T, store *readingprogress.Store, shelfID, bookID string) int64 {
	t.Helper()
	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	return doc.Shelves[shelfID][bookID].Offset
}

func TestPersistOnClose_WritesStagedPosition(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	s := NewStager(store, time.Second)

	s.Stage("real-shelf", "book-a", 420, 1000)
	s.PersistOnClose()

	if got := offsetOf(t, store, "real-shelf", "book-a"); got != 420 {
		t.Fatalf("persisted offset = %d, want 420", got)
	}
}

func TestPersistOnCloseKeepsNewerEntry(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	if _, err := store.Mutate(func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves["real-shelf"] = map[string]readingprogress.Entry{"book-a": {Offset: 900, At: 5000}}
		return out
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := NewStager(store, time.Second)
	s.Stage("real-shelf", "book-a", 100, 1000)
	s.PersistOnClose()

	if got := offsetOf(t, store, "real-shelf", "book-a"); got != 900 {
		t.Fatalf("offset = %d, want 900 (newer on-disk entry must survive)", got)
	}
}

func TestPersistOnClose_EmptyIsNoop(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	s := NewStager(store, time.Second)

	s.PersistOnClose()

	if _, raw, err := store.Read(); err != nil || raw != "" {
		t.Fatalf("empty close wrote %q (err %v), want no file", raw, err)
	}
}
