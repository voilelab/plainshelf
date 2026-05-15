package store

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(t.TempDir(), 100)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetBookmark_NotFound(t *testing.T) {
	db := newTestDB(t)
	mark, err := db.GetBookmark("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mark.CharOffset != 0 {
		t.Fatalf("expected 0, got %d", mark.CharOffset)
	}
}

func TestSetBookmark(t *testing.T) {
	db := newTestDB(t)
	if err := db.SetBookmark("book1", Bookmark{CharOffset: 42}); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}
	mark, err := db.GetBookmark("book1")
	if err != nil {
		t.Fatalf("GetBookmark: %v", err)
	}
	if mark.CharOffset != 42 {
		t.Fatalf("expected 42, got %d", mark.CharOffset)
	}
}

func TestSet_OverwriteBookmark(t *testing.T) {
	db := newTestDB(t)
	db.SetBookmark("book1", Bookmark{CharOffset: 10})
	if err := db.SetBookmark("book1", Bookmark{CharOffset: 99}); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}
	mark, err := db.GetBookmark("book1")
	if err != nil {
		t.Fatalf("GetBookmark: %v", err)
	}
	if mark.CharOffset != 99 {
		t.Fatalf("expected 99, got %d", mark.CharOffset)
	}
}

func TestSet_MultipleBooks(t *testing.T) {
	db := newTestDB(t)
	books := map[string]int{"a": 1, "b": 2, "c": 3}
	for id, pos := range books {
		if err := db.SetBookmark(id, Bookmark{CharOffset: pos}); err != nil {
			t.Fatalf("SetBookmark %q: %v", id, err)
		}
	}
	for id, want := range books {
		got, err := db.GetBookmark(id)
		if err != nil {
			t.Fatalf("Get %q: %v", id, err)
		}
		if got.CharOffset != want {
			t.Fatalf("book %q: expected %d, got %d", id, want, got.CharOffset)
		}
	}
}
