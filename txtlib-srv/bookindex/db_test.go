package bookindex

import (
	"sort"
	"testing"
)

func assertIDsEqual(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("unexpected result length: got %d, want %d (got=%v, want=%v)", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected result at %d: got %q, want %q (got=%v, want=%v)", i, got[i], want[i], got, want)
		}
	}
}

func TestDBAddAndHas(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	ok := db.Add("book-1", MetaMap{"author": "Alice", "genre": "SciFi"})
	if !ok {
		t.Fatalf("expected Add to succeed")
	}
	if !db.Has("book-1") {
		t.Fatalf("expected Has to return true for existing item")
	}
	if db.Has("missing") {
		t.Fatalf("expected Has to return false for missing item")
	}
}

func TestDBAddAndQuery(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	ok := db.Add("book-1", MetaMap{"author": "Alice", "genre": "SciFi"})
	if !ok {
		t.Fatalf("expected Add to succeed")
	}

	assertIDsEqual(t, db.Query("author", "Alice"), []string{"book-1"})
	assertIDsEqual(t, db.Query("genre", "SciFi"), []string{"book-1"})
}

func TestDBAddDuplicate(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	if !db.Add("book-1", MetaMap{"author": "Alice"}) {
		t.Fatalf("initial Add should succeed")
	}
	if db.Add("book-1", MetaMap{"author": "Bob"}) {
		t.Fatalf("duplicate Add should fail")
	}

	// Metadata should remain from the first insertion.
	assertIDsEqual(t, db.Query("author", "Alice"), []string{"book-1"})
	assertIDsEqual(t, db.Query("author", "Bob"), []string{})
}

func TestDBRemove(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	db.Add("book-1", MetaMap{"author": "Alice", "tag": "fav"})
	if !db.Remove("book-1") {
		t.Fatalf("expected Remove to succeed")
	}
	if db.Remove("book-1") {
		t.Fatalf("expected removing missing item to fail")
	}

	assertIDsEqual(t, db.Query("author", "Alice"), []string{})
	assertIDsEqual(t, db.Query("tag", "fav"), []string{})
}

func TestDBUpdate(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	if !db.Add("book-1", MetaMap{"author": "Alice", "genre": "SciFi"}) {
		t.Fatalf("initial Add should succeed")
	}
	db.Update("book-1", MetaMap{"author": "Bob", "genre": "Drama"})
	db.Update("missing", MetaMap{"author": "Nobody"})

	assertIDsEqual(t, db.Query("author", "Alice"), []string{})
	assertIDsEqual(t, db.Query("author", "Bob"), []string{"book-1"})
	assertIDsEqual(t, db.Query("genre", "SciFi"), []string{})
	assertIDsEqual(t, db.Query("genre", "Drama"), []string{"book-1"})
}

func TestDBQueryNoMatch(t *testing.T) {
	dbPath := t.TempDir()
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	db.Add("book-1", MetaMap{"author": "Alice"})

	assertIDsEqual(t, db.Query("missing", "value"), []string{})
	assertIDsEqual(t, db.Query("author", "missing"), []string{})
}

func TestDBPersistence(t *testing.T) {
	dbPath := t.TempDir()

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create persistent db: %v", err)
	}
	defer db.Close()

	if !db.Add("book-1", MetaMap{"author": "Alice", "genre": "SciFi"}) {
		t.Fatalf("initial Add should succeed")
	}
	if !db.Add("book-2", MetaMap{"author": "Bob", "genre": "Drama"}) {
		t.Fatalf("second Add should succeed")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("failed to close persistent db: %v", err)
	}

	reopened, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen persistent db: %v", err)
	}
	defer reopened.Close()

	assertIDsEqual(t, reopened.Query("author", "Alice"), []string{"book-1"})
	assertIDsEqual(t, reopened.Query("genre", "Drama"), []string{"book-2"})
}
