package bookmark

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

func TestGet_NotFound(t *testing.T) {
	db := newTestDB(t)
	mark, err := db.Get("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mark.CharOffset != 0 {
		t.Fatalf("expected 0, got %d", mark.CharOffset)
	}
}

func TestSetGet(t *testing.T) {
	db := newTestDB(t)
	if err := db.Set("book1", Mark{CharOffset: 42}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	mark, err := db.Get("book1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mark.CharOffset != 42 {
		t.Fatalf("expected 42, got %d", mark.CharOffset)
	}
}

func TestSet_Overwrite(t *testing.T) {
	db := newTestDB(t)
	db.Set("book1", Mark{CharOffset: 10})
	if err := db.Set("book1", Mark{CharOffset: 99}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	mark, err := db.Get("book1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mark.CharOffset != 99 {
		t.Fatalf("expected 99, got %d", mark.CharOffset)
	}
}

func TestSet_MultipleBooks(t *testing.T) {
	db := newTestDB(t)
	books := map[string]int{"a": 1, "b": 2, "c": 3}
	for id, pos := range books {
		if err := db.Set(id, Mark{CharOffset: pos}); err != nil {
			t.Fatalf("Set %q: %v", id, err)
		}
	}
	for id, want := range books {
		got, err := db.Get(id)
		if err != nil {
			t.Fatalf("Get %q: %v", id, err)
		}
		if got.CharOffset != want {
			t.Fatalf("book %q: expected %d, got %d", id, want, got.CharOffset)
		}
	}
}
