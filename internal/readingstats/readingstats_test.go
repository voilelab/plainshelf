package readingstats

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 1, 15, 0, 0, 0, time.Local)

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
	return NewStore(filepath.Join(t.TempDir(), "reading_stats.json"))
}

func TestConcurrentStoresSumEverySecond(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reading_stats.json")
	stores := []*Store{NewStore(path), NewStore(path)}
	const perStore = 25

	var wg sync.WaitGroup
	errs := make(chan error, len(stores)*perStore)
	for s, store := range stores {
		for range perStore {
			wg.Go(func() {
				errs <- store.Add("main", "2026-11-01", float64(s+1), now)
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("add: %v", err)
		}
	}

	if got, want := readDoc(t, stores[0]).Shelves["main"]["2026-11-01"], int64(perStore*1+perStore*2); got != want {
		t.Fatalf("stored %d seconds, want %d", got, want)
	}
}

func TestAddClampsPerCallAndPerDay(t *testing.T) {
	store := newStore(t)
	if err := store.Add("main", "2026-11-01", 500, now); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := readDoc(t, store).Shelves["main"]["2026-11-01"]; got != maxSecondsPerCall {
		t.Fatalf("one call stored %d seconds, want %d", got, maxSecondsPerCall)
	}

	near := `{"version":1,"shelves":{"main":{"2026-11-01":86390}}}`
	got := withSeconds(Parse(near), "main", "2026-11-01", 60, now)
	if got.Shelves["main"]["2026-11-01"] != maxSecondsPerDay {
		t.Fatalf("day total = %d, want %d", got.Shelves["main"]["2026-11-01"], maxSecondsPerDay)
	}
}

func TestAddIgnoresInvalidInput(t *testing.T) {
	doc := New()
	cases := []struct {
		key, date string
		seconds   float64
	}{
		{"main", "2026-11-01", 0},
		{"main", "2026-11-01", -5},
		{"main", "2026-11-01", 0.4},
		{"", "2026-11-01", 30},
		{"main", "2026-02-30", 30},
		{"main", "2026-1-01", 30},
		{"main", "2026-11-02", 30},
		{"main", "2025-09-27", 30},
	}
	for _, c := range cases {
		if got := withSeconds(doc, c.key, c.date, c.seconds, now); len(got.Shelves) != 0 {
			t.Errorf("withSeconds(%q, %q, %v) stored %v", c.key, c.date, c.seconds, got.Shelves)
		}
	}
}

func TestAddPrunesDaysOutsideRetention(t *testing.T) {
	stored := `{"version":1,"shelves":{"main":{"2025-09-27":10,"2025-09-28":20},"other":{"2020-01-01":5}}}`
	got := withSeconds(Parse(stored), "main", "2026-11-01", 30, now)

	want := map[string]int64{"2025-09-28": 20, "2026-11-01": 30}
	if len(got.Shelves["main"]) != len(want) {
		t.Fatalf("main = %v, want %v", got.Shelves["main"], want)
	}
	for date, seconds := range want {
		if got.Shelves["main"][date] != seconds {
			t.Fatalf("main = %v, want %v", got.Shelves["main"], want)
		}
	}
	if got.Shelves["other"]["2020-01-01"] != 5 {
		t.Fatalf("another key was pruned: %v", got.Shelves["other"])
	}
}

func TestParseMatchesFrontendNormalization(t *testing.T) {
	stored := `{"version":1,"shelves":{"main":{"2026-11-01":12.6,"2026-02-30":5,"bad":5,"2026-10-31":"7","2026-10-30":-1,"2026-10-29":100000},"empty":{}}}`
	got, err := Serialize(Parse(stored))
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if want := `{"version":1,"shelves":{"main":{"2026-10-29":86400,"2026-11-01":13}}}`; got != want {
		t.Fatalf("parse = %s, want %s", got, want)
	}

	for _, text := range []string{"", "{oops", "[1]", `{"version":2,"shelves":{"main":{"2026-11-01":5}}}`} {
		if doc := Parse(text); len(doc.Shelves) != 0 {
			t.Errorf("Parse(%q) = %v, want empty", text, doc.Shelves)
		}
	}
}

func TestStoreNeedsPath(t *testing.T) {
	store := NewStore("")
	if _, err := store.Read(); err == nil {
		t.Fatal("read succeeded without a path")
	}
	if err := store.Add("main", "2026-11-01", 30, now); err == nil {
		t.Fatal("add succeeded without a path")
	}
}
