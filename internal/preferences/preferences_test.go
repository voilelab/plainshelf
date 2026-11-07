package preferences

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "preferences.json")
}

func mustRead(t *testing.T, store *Store) map[string]string {
	t.Helper()
	values, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return values
}

func TestConcurrentStoresKeepEveryKey(t *testing.T) {
	path := newPath(t)
	stores := []*Store{NewStore(path, AppDesktop), NewStore(path, AppReader)}
	const perStore = 25

	var wg sync.WaitGroup
	errs := make(chan error, len(stores)*perStore)
	for s, store := range stores {
		for i := range perStore {
			wg.Go(func() {
				errs <- store.Set(fmt.Sprintf("key-%d-%d", s, i), fmt.Sprintf("value-%d-%d", s, i))
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("set: %v", err)
		}
	}

	got := mustRead(t, stores[0])
	for s := range stores {
		for i := range perStore {
			key := fmt.Sprintf("key-%d-%d", s, i)
			if got[key] != fmt.Sprintf("value-%d-%d", s, i) {
				t.Fatalf("lost %s: %v", key, got)
			}
		}
	}
}

func TestSharedKeyVisibleToBothApps(t *testing.T) {
	path := newPath(t)
	if err := NewStore(path, AppReader).Set("reader-font-size", "24"); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, NewStore(path, AppDesktop))["reader-font-size"]; got != "24" {
		t.Fatalf("desktop reader-font-size = %q, want 24", got)
	}
}

func TestShelfKeyIsPerApp(t *testing.T) {
	path := newPath(t)
	desktop, reader := NewStore(path, AppDesktop), NewStore(path, AppReader)
	if err := desktop.Set("plainshelf.shelf", "main"); err != nil {
		t.Fatal(err)
	}
	if err := reader.Set("plainshelf.shelf", "standalone"); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, desktop)["plainshelf.shelf"]; got != "main" {
		t.Fatalf("desktop shelf = %q, want main", got)
	}
	if got := mustRead(t, reader)["plainshelf.shelf"]; got != "standalone" {
		t.Fatalf("reader shelf = %q, want standalone", got)
	}
}

func TestEmptyValueDeletesKey(t *testing.T) {
	store := NewStore(newPath(t), AppDesktop)
	for _, key := range []string{"plainshelf.shelf", "plainshelf.locale"} {
		if err := store.Set(key, "x"); err != nil {
			t.Fatal(err)
		}
		if err := store.Set(key, ""); err != nil {
			t.Fatal(err)
		}
		if _, ok := mustRead(t, store)[key]; ok {
			t.Fatalf("%s still present after delete", key)
		}
	}
}

func TestSetRejectsInvalidKey(t *testing.T) {
	store := NewStore(newPath(t), AppDesktop)
	for _, key := range []string{"", " padded", string(make([]byte, maxKeyBytes+1))} {
		if err := store.Set(key, "v"); err == nil {
			t.Fatalf("Set(%q) accepted", key)
		}
	}
	if err := store.Set("k", string(make([]byte, maxValueBytes+1))); err == nil {
		t.Fatal("oversized value accepted")
	}
}

func TestParseDropsMalformedEntries(t *testing.T) {
	path := newPath(t)
	text := `{"version":1,"shared":{"a":"1","b":2,"":"x"},"apps":{"desktop":{"plainshelf.shelf":"main","stray":"y"}}}`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, NewStore(path, AppDesktop))
	if len(got) != 2 || got["a"] != "1" || got["plainshelf.shelf"] != "main" {
		t.Fatalf("values = %v, want a and plainshelf.shelf only", got)
	}
}

func TestParseResetsUnknownVersion(t *testing.T) {
	path := newPath(t)
	if err := os.WriteFile(path, []byte(`{"version":2,"shared":{"a":"1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, NewStore(path, AppDesktop)); len(got) != 0 {
		t.Fatalf("values = %v, want empty", got)
	}
}

func TestReadWithoutPathFails(t *testing.T) {
	if _, err := NewStore("", AppDesktop).Read(); err == nil {
		t.Fatal("read without a path succeeded")
	}
}
