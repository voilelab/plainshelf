package store

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetBookmark_NotFound(t *testing.T) {
	db := newTestDB(t)
	dbID := "test_shelf"
	mark, err := db.GetBookmark(dbID, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mark.CharOffset != 0 {
		t.Fatalf("expected 0, got %d", mark.CharOffset)
	}
}

func TestSetBookmark(t *testing.T) {
	db := newTestDB(t)
	dbID := "test_shelf"
	if err := db.SetBookmark(dbID, "book1", Bookmark{CharOffset: 42}); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}
	mark, err := db.GetBookmark(dbID, "book1")
	if err != nil {
		t.Fatalf("GetBookmark: %v", err)
	}
	if mark.CharOffset != 42 {
		t.Fatalf("expected 42, got %d", mark.CharOffset)
	}

	// Advancing the reading position is the common case, so a second write to
	// the same book must replace the offset rather than keep the first one.
	if err := db.SetBookmark(dbID, "book1", Bookmark{CharOffset: 99}); err != nil {
		t.Fatalf("SetBookmark (overwrite): %v", err)
	}
	mark, err = db.GetBookmark(dbID, "book1")
	if err != nil {
		t.Fatalf("GetBookmark (overwrite): %v", err)
	}
	if mark.CharOffset != 99 {
		t.Fatalf("expected 99 after overwrite, got %d", mark.CharOffset)
	}
}
