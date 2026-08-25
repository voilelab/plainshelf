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

func ent(offset, at int64) Entry { return Entry{Offset: offset, At: at} }

func entryOf(doc Document, shelfKey, bookID string) Entry {
	return doc.Shelves[shelfKey][bookID]
}

func TestProjectMapsReaderBookToRealShelf(t *testing.T) {
	// book-id mapping: an entry under the synthetic "book" shelf is folded into
	// the correct real shelf, and the reader entry is kept.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(120, 1000)}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

	if e := entryOf(got, "shelf-real", "book-a"); e != ent(120, 1000) {
		t.Fatalf("real shelf entry = %+v, want {120 1000}", e)
	}
	if e := entryOf(got, ReaderShelfID, "book-a"); e != ent(120, 1000) {
		t.Fatalf("reader entry = %+v, want {120 1000} (must be kept)", e)
	}
}

func TestProjectShelfIDDifference(t *testing.T) {
	// The synthetic reader shelf key is never equal to a real shelf key, yet the
	// book still maps to the real shelf.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(5, 1000)}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "abcdef234567"}))

	if _, ok := got.Shelves["abcdef234567"]["book-a"]; !ok {
		t.Fatalf("expected book-a under the real shelf key")
	}
	if ReaderShelfID == "abcdef234567" {
		t.Fatalf("test precondition: reader shelf id must differ from the real shelf id")
	}
}

func TestProjectConflictTakesNewest(t *testing.T) {
	cases := []struct {
		name       string
		existing   Entry
		reader     Entry
		wantOffset int64
	}{
		{"existing newer: keep existing", ent(300, 2000), ent(100, 1000), 300},
		{"reader newer: take reader", ent(100, 1000), ent(300, 2000), 300},
		{"equal timestamp: incoming reader wins", ent(200, 1500), ent(999, 1500), 999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := New()
			doc.Shelves["shelf-real"] = map[string]Entry{"book-a": tc.existing}
			doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": tc.reader}

			got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

			if v := entryOf(got, "shelf-real", "book-a").Offset; v != tc.wantOffset {
				t.Fatalf("real shelf offset = %d, want %d", v, tc.wantOffset)
			}
		})
	}
}

func TestProjectUnresolvedStaysUnderReaderShelf(t *testing.T) {
	// Reverse condition: a book id no real shelf resolves (removed book, or a
	// loose book opened from outside every shelf) is not projected anywhere and
	// is left untouched under the reader shelf.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-loose": ent(77, 1000)}

	got := Project(doc, ReaderShelfID, resolverFrom(map[string]string{}))

	if e := entryOf(got, ReaderShelfID, "book-loose"); e != ent(77, 1000) {
		t.Fatalf("loose reader entry = %+v, want {77 1000} (kept)", e)
	}
	if len(got.Shelves) != 1 {
		t.Fatalf("unresolved book must not create any real-shelf entry; shelves = %v", got.Shelves)
	}
}

func TestProjectIsIdempotent(t *testing.T) {
	doc := New()
	doc.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(40, 500)}
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(90, 1000), "book-loose": ent(10, 800)}
	resolve := resolverFrom(map[string]string{"book-a": "shelf-real"})

	once := Project(doc, ReaderShelfID, resolve)
	twice := Project(once, ReaderShelfID, resolve)

	if !reflect.DeepEqual(once, twice) {
		t.Fatalf("projection is not idempotent:\n once=%v\n twice=%v", once, twice)
	}
	if e := entryOf(twice, "shelf-real", "book-a"); e != ent(90, 1000) {
		t.Fatalf("folded entry = %+v, want {90 1000}", e)
	}
}

func TestProjectDeferredReHome(t *testing.T) {
	// A book has reader progress before any real shelf holds it; a later import
	// makes it resolvable, and the next projection folds it in.
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(55, 1000)}

	before := Project(doc, ReaderShelfID, resolverFrom(map[string]string{}))
	if _, ok := before.Shelves["shelf-real"]; ok {
		t.Fatalf("book must stay unprojected while unresolved")
	}

	after := Project(before, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))
	if e := entryOf(after, "shelf-real", "book-a"); e != ent(55, 1000) {
		t.Fatalf("re-homed entry = %+v, want {55 1000}", e)
	}
	if e := entryOf(after, ReaderShelfID, "book-a"); e != ent(55, 1000) {
		t.Fatalf("reader entry must remain after re-home, got %+v", e)
	}
}

func TestProjectDoesNotMutateInput(t *testing.T) {
	doc := New()
	doc.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(30, 1000)}

	_ = Project(doc, ReaderShelfID, resolverFrom(map[string]string{"book-a": "shelf-real"}))

	if _, ok := doc.Shelves["shelf-real"]; ok {
		t.Fatalf("Project must not mutate its input document")
	}
}

func TestMergeNewestKeepsTheNewerEntryPerBook(t *testing.T) {
	disk := New()
	// A reader (launched from desktop) advanced the real shelf at t=2000; a second
	// reader book lives under the synthetic shelf.
	disk.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(20, 2000)}
	disk.Shelves[ReaderShelfID] = map[string]Entry{"book-b": ent(200, 2000)}

	// A desktop write whose in-memory copy of the real shelf is stale (t=1000) and
	// that adds progress for a different book.
	write := New()
	write.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(10, 1000), "book-c": ent(5, 3000)}

	got := MergeNewest(disk, write)

	if e := entryOf(got, "shelf-real", "book-a"); e != ent(20, 2000) {
		t.Fatalf("stale desktop write must not clobber the newer reader entry: %+v, want {20 2000}", e)
	}
	if e := entryOf(got, "shelf-real", "book-c"); e != ent(5, 3000) {
		t.Fatalf("write-only book must be added: %+v, want {5 3000}", e)
	}
	if e := entryOf(got, ReaderShelfID, "book-b"); e != ent(200, 2000) {
		t.Fatalf("disk-only book must be preserved: %+v, want {200 2000}", e)
	}
}

func TestMergeNewestTombstoneWinsByRecency(t *testing.T) {
	disk := New()
	disk.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(120, 1000)}

	// A reset written later: a zero-offset tombstone with a newer timestamp.
	write := New()
	write.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(0, 2000)}

	got := MergeNewest(disk, write)
	if e := entryOf(got, "shelf-real", "book-a"); e != ent(0, 2000) {
		t.Fatalf("newer reset tombstone must win: %+v, want {0 2000}", e)
	}

	// An older reset must not undo a newer advance.
	advance := New()
	advance.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(120, 3000)}
	oldReset := New()
	oldReset.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(0, 1500)}
	got2 := MergeNewest(advance, oldReset)
	if e := entryOf(got2, "shelf-real", "book-a"); e != ent(120, 3000) {
		t.Fatalf("newer advance must survive an older reset: %+v, want {120 3000}", e)
	}
}

func TestMergeNewestDoesNotMutateInputs(t *testing.T) {
	disk := New()
	disk.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(10, 1000)}
	write := New()
	write.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(20, 2000)}

	_ = MergeNewest(disk, write)

	if e := entryOf(disk, "shelf-real", "book-a"); e != ent(10, 1000) {
		t.Fatalf("MergeNewest mutated the disk document: %+v", e)
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
		{"older version", `{"version":1,"shelves":{"s":{"b":5}}}`},
		{"newer version", `{"version":3,"shelves":{"s":{"b":{"offset":5,"at":1}}}}`},
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

func TestParseDropsInvalidEntries(t *testing.T) {
	doc := Parse(`{"version":2,"shelves":{"s":{` +
		`"ok":{"offset":5,"at":10},` +
		`"tombstone":{"offset":0,"at":20},` +
		`"bare-zero":{"offset":0,"at":0},` +
		`"neg":{"offset":-3,"at":10},` +
		`"frac":{"offset":1.5,"at":10}` +
		`},"empty":{}}}`)
	shelf, ok := doc.Shelves["s"]
	if !ok {
		t.Fatalf("shelf s missing")
	}
	if e, ok := shelf["ok"]; !ok || e != ent(5, 10) {
		t.Fatalf("valid entry dropped or wrong: %+v %v", e, ok)
	}
	if e, ok := shelf["tombstone"]; !ok || e != ent(0, 20) {
		t.Fatalf("reset tombstone must be kept: %+v %v", e, ok)
	}
	for _, bad := range []string{"bare-zero", "neg", "frac"} {
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
	doc.Shelves["shelf-real"] = map[string]Entry{"book-a": ent(42, 1700000000000)}
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
	if text != `{"version":2,"shelves":{}}` {
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
		out.Shelves[ReaderShelfID] = map[string]Entry{"book-a": ent(12, 1000)}
		return out
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}

	reloaded, _, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if e := entryOf(reloaded, ReaderShelfID, "book-a"); e != ent(12, 1000) {
		t.Fatalf("persisted entry = %+v, want {12 1000}", e)
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
