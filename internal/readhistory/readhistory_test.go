package readhistory

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func readDoc(t *testing.T, store *Store) Document {
	t.Helper()
	raw, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return Parse(raw)
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "read_history.json"))
}

func TestConcurrentStoresKeepEveryRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read_history.json")
	stores := []*Store{NewStore(path), NewStore(path)}
	const perStore = 25

	var wg sync.WaitGroup
	errs := make(chan error, len(stores)*perStore)
	for s, store := range stores {
		for i := range perStore {
			wg.Go(func() {
				errs <- store.Record("main", fmt.Sprintf("book-%d-%d", s, i))
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	got := readDoc(t, stores[0]).Shelves["main"]
	if len(got) != len(stores)*perStore {
		t.Fatalf("stored %d books, want %d: %v", len(got), len(stores)*perStore, got)
	}
	for s := range stores {
		for i := range perStore {
			if id := fmt.Sprintf("book-%d-%d", s, i); !slices.Contains(got, id) {
				t.Fatalf("lost %s from %v", id, got)
			}
		}
	}
}

func TestRecordMovesBookToFront(t *testing.T) {
	store := newStore(t)
	for _, id := range []string{"book-1", "book-2", " book-1 "} {
		if err := store.Record("main", id); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	if got := readDoc(t, store).Shelves["main"]; !slices.Equal(got, []string{"book-1", "book-2"}) {
		t.Fatalf("history = %v", got)
	}
}

func TestRecordTrimsToLimit(t *testing.T) {
	store := newStore(t)
	if err := store.SetLimit(2); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	for _, id := range []string{"book-1", "book-2", "book-3"} {
		if err := store.Record("main", id); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	if got := readDoc(t, store).Shelves["main"]; !slices.Equal(got, []string{"book-3", "book-2"}) {
		t.Fatalf("history = %v", got)
	}
}

func TestRecordSkipsBlankIDAndZeroLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read_history.json")
	store := NewStore(path)
	if err := store.Record("main", "  "); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a blank id created the document: %v", err)
	}

	if err := store.SetLimit(0); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := store.Record("main", "book-1"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if got := readDoc(t, store).Shelves; len(got) != 0 {
		t.Fatalf("history with limit 0 = %v", got)
	}
}

func TestClearOnlyTouchesOneShelf(t *testing.T) {
	store := newStore(t)
	_ = store.Record("a", "book-1")
	_ = store.Record("b", "book-2")

	if err := store.Clear("a"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	got := readDoc(t, store).Shelves
	if _, ok := got["a"]; ok || !slices.Equal(got["b"], []string{"book-2"}) {
		t.Fatalf("shelves after clear = %v", got)
	}
}

func TestSetLimitTrimsStoredShelves(t *testing.T) {
	store := newStore(t)
	_ = store.Record("main", "book-1")
	_ = store.Record("main", "book-2")

	if err := store.SetLimit(1); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	doc := readDoc(t, store)
	if doc.Limit != 1 || !slices.Equal(doc.Shelves["main"], []string{"book-2"}) {
		t.Fatalf("doc = %+v", doc)
	}

	if err := store.SetLimit(-3); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if got := readDoc(t, store).Limit; got != DefaultLimit {
		t.Fatalf("negative limit stored as %d, want %d", got, DefaultLimit)
	}
}

func TestParseMatchesFrontendNormalization(t *testing.T) {
	cases := map[string]Document{
		"":        New(),
		"{oops":   New(),
		"[1,2,3]": New(),
		`"text"`:  New(),
		`{"version":99,"limit":5,"shelves":{"a":["b"]}}`: New(),
		`{"version":1,"limit":2,"shelves":{"s":["book-1",7,""," book-1 ","book-2","book-3"]}}`: {
			Version: 1, Limit: 2, Shelves: map[string][]string{"s": {"book-1", "book-2"}},
		},
		`{"version":1,"limit":-3,"shelves":["book-1"]}`: New(),
		`{"version":1,"limit":1.5,"shelves":{}}`:        New(),
	}
	for text, want := range cases {
		got := Parse(text)
		gotText, _ := Serialize(got)
		wantText, _ := Serialize(want)
		if gotText != wantText {
			t.Errorf("Parse(%q) = %s, want %s", text, gotText, wantText)
		}
	}
}

func TestSerializeMatchesFrontendShape(t *testing.T) {
	doc := withEntry(New(), "main", "book-1")
	text, err := Serialize(doc)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if want := `{"version":1,"limit":100,"shelves":{"main":["book-1"]}}`; text != want {
		t.Fatalf("serialize = %s, want %s", text, want)
	}
}

func TestStoreNeedsPath(t *testing.T) {
	store := NewStore("")
	if _, err := store.Read(); err == nil {
		t.Fatal("read succeeded without a path")
	}
	if err := store.Record("main", "book-1"); err == nil {
		t.Fatal("record succeeded without a path")
	}
}
