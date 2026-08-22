package readingprogress

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// resolverFrom builds a ResolveShelf from a bookID -> realShelfID map. A book id
// absent from the map is unresolvable (the reverse condition).
func resolverFrom(m map[string]string) ResolveShelf {
	return func(bookID string) (string, bool) {
		shelfID, ok := m[bookID]
		return shelfID, ok
	}
}

func offset(doc Document, shelfKey, bookID string) int64 {
	return doc.Shelves[shelfKey][bookID]
}

func TestProjectMapsReaderBookToRealShelf(t *testing.T) {
	// book-id mapping: an offset under the synthetic "book" shelf is folded into
	// the correct real shelf, and the reader entry is kept.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": 120}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

	if v := offset(got, "shelf-real", "book-a"); v != 120 {
		t.Fatalf("real shelf offset = %d, want 120", v)
	}
	if v := offset(got, ReaderShelfID, "book-a"); v != 120 {
		t.Fatalf("reader entry offset = %d, want 120 (must be kept)", v)
	}
}

func TestProjectShelfIDDifference(t *testing.T) {
	// The synthetic reader shelf key is never equal to a real shelf key, yet the
	// book still maps to the real shelf.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": 5}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "abcdef234567"}))

	if _, ok := got.Shelves["abcdef234567"]["book-a"]; !ok {
		t.Fatalf("expected book-a under the real shelf key")
	}
	if ReaderShelfID == "abcdef234567" {
		t.Fatalf("test precondition: reader shelf id must differ from the real shelf id")
	}
}

func TestProjectConflictTakesMax(t *testing.T) {
	cases := []struct {
		name     string
		existing int64
		reader   int64
		want     int64
	}{
		{"existing ahead: keep existing", 300, 100, 300},
		{"reader ahead: take reader", 100, 300, 300},
		{"equal: unchanged", 200, 200, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := New()
			doc.Shelves["shelf-real"] = map[string]int64{"book-a": tc.existing}
			doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": tc.reader}

			got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

			if v := offset(got, "shelf-real", "book-a"); v != tc.want {
				t.Fatalf("real shelf offset = %d, want %d", v, tc.want)
			}
		})
	}
}

func TestProjectUnresolvedStaysUnderReaderShelf(t *testing.T) {
	// Reverse condition: a book id no real shelf resolves (removed book, or a
	// loose book opened from outside every shelf) is not projected anywhere and
	// is left untouched under the reader shelf.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-loose": 77}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{}))

	if v := offset(got, ReaderShelfID, "book-loose"); v != 77 {
		t.Fatalf("loose reader entry offset = %d, want 77 (kept)", v)
	}
	if len(got.Shelves) != 1 {
		t.Fatalf("unresolved book must not create any real-shelf entry; shelves = %v", got.Shelves)
	}
}

func TestProjectIsIdempotent(t *testing.T) {
	doc := New()
	doc.Shelves["shelf-real"] = map[string]int64{"book-a": 40}
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": 90, "book-loose": 10}
	resolve := resolverFrom(map[string]string{"book-a": "shelf-real"})

	once := Project(doc, ReaderShelfID, resolve)
	twice := Project(once, ReaderShelfID, resolve)

	if !reflect.DeepEqual(once, twice) {
		t.Fatalf("projection is not idempotent:\n once=%v\n twice=%v", once, twice)
	}
	if v := offset(twice, "shelf-real", "book-a"); v != 90 {
		t.Fatalf("folded offset = %d, want 90", v)
	}
}

func TestProjectDeferredReHome(t *testing.T) {
	// A book has reader progress before any real shelf holds it; a later import
	// makes it resolvable, and the next projection folds it in.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": 55}

	before := Project(doc, ReaderShelfID, resolverFrom(map[string]string{}))
	if _, ok := before.Shelves["shelf-real"]; ok {
		t.Fatalf("book must stay unprojected while unresolved")
	}

	after := Project(before, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))
	if v := offset(after, "shelf-real", "book-a"); v != 55 {
		t.Fatalf("re-homed offset = %d, want 55", v)
	}
	if v := offset(after, ReaderShelfID, "book-a"); v != 55 {
		t.Fatalf("reader entry must remain after re-home, got %d", v)
	}
}

func TestProjectDoesNotMutateInput(t *testing.T) {
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]int64{"book-a": 30}

	_ = Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

	if _, ok := doc.Shelves["shelf-real"]; ok {
		t.Fatalf("Project must not mutate its input document")
	}
}

func TestMergeReaderWriteKeepsDesktopNamespaces(t *testing.T) {
	disk := New()
	disk.Shelves["shelf-real"] = map[string]int64{"book-a": 500}
	disk.Shelves[ReaderShelfID] = map[string]int64{"book-a": 100}

	// A reader write that only advanced its own "book" namespace.
	write := New()
	write.Shelves[ReaderShelfID] = map[string]int64{"book-a": 150}
	// A stale real-shelf entry the reader frontend may have carried in memory;
	// it must be ignored.
	write.Shelves["shelf-real"] = map[string]int64{"book-a": 1}

	got := MergeReaderWrite(disk, write, ReaderShelfID)

	if v := offset(got, ReaderShelfID, "book-a"); v != 150 {
		t.Fatalf("reader namespace offset = %d, want 150", v)
	}
	if v := offset(got, "shelf-real", "book-a"); v != 500 {
		t.Fatalf("desktop namespace offset = %d, want 500 (reader must not clobber it)", v)
	}
}

func TestMergeDesktopWriteKeepsReaderNamespace(t *testing.T) {
	disk := New()
	disk.Shelves["shelf-real"] = map[string]int64{"book-a": 200}
	disk.Shelves[ReaderShelfID] = map[string]int64{"book-a": 999}

	// A desktop write that advanced a real shelf; its in-memory copy of the
	// reader namespace is stale and must not win.
	write := New()
	write.Shelves["shelf-real"] = map[string]int64{"book-a": 260}
	write.Shelves[ReaderShelfID] = map[string]int64{"book-a": 1}

	got := MergeDesktopWrite(disk, write, ReaderShelfID)

	if v := offset(got, "shelf-real", "book-a"); v != 260 {
		t.Fatalf("desktop namespace offset = %d, want 260", v)
	}
	if v := offset(got, ReaderShelfID, "book-a"); v != 999 {
		t.Fatalf("reader namespace offset = %d, want 999 (desktop must not clobber it)", v)
	}
}

func TestParseLeniency(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"not json", "{not json"},
		{"array", "[]"},
		{"wrong version", `{"version":2,"shelves":{"s":{"b":5}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := Parse(tc.text)
			if doc.Version != DocumentVersion || len(doc.Shelves) != 0 {
				t.Fatalf("expected empty document, got %v", doc)
			}
		})
	}
}

func TestParseDropsInvalidOffsets(t *testing.T) {
	doc := Parse(`{"version":1,"shelves":{"s":{"ok":5,"zero":0,"neg":-3,"frac":1.5},"empty":{}}}`)
	shelf, ok := doc.Shelves["s"]
	if !ok {
		t.Fatalf("shelf s missing")
	}
	if v, ok := shelf["ok"]; !ok || v != 5 {
		t.Fatalf("valid offset dropped or wrong: %d %v", v, ok)
	}
	for _, bad := range []string{"zero", "neg", "frac"} {
		if _, ok := shelf[bad]; ok {
			t.Fatalf("expected %q to be dropped", bad)
		}
	}
	if _, ok := doc.Shelves["empty"]; ok {
		t.Fatalf("empty shelf must be dropped")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	doc := New()
	doc.Shelves["shelf-real"] = map[string]int64{"book-a": 42}
	text, err := Serialize(doc)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if got := Parse(text); !reflect.DeepEqual(got, doc) {
		t.Fatalf("round trip mismatch:\n got=%v\n want=%v", got, doc)
	}
}

func TestSerializeEmptyShelvesIsObjectNotNull(t *testing.T) {
	text, err := Serialize(Document{})
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if text != `{"version":1,"shelves":{}}` {
		t.Fatalf("unexpected serialization: %s", text)
	}
}

func TestStoreMutateReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reading_progress.json")
	store := NewStore(path)

	// Missing file reads as an empty document.
	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(doc.Shelves) != 0 {
		t.Fatalf("expected empty document, got %v", doc)
	}

	// A mutation persists.
	_, err = store.Mutate(func(d Document) Document {
		out := d.Clone()
		out.Shelves[ReaderShelfID] = map[string]int64{"book-a": 12}
		return out
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}

	reloaded, _, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if v := offset(reloaded, ReaderShelfID, "book-a"); v != 12 {
		t.Fatalf("persisted offset = %d, want 12", v)
	}
}

func TestStoreMutateNoOpDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reading_progress.json")
	store := NewStore(path)

	// An identity mutation on a missing file must not create the file.
	if _, err := store.Mutate(func(d Document) Document { return d }); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if _, _, err := store.Read(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if fileExists(path) {
		t.Fatalf("no-op mutation must not create the document file")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
