package shelf

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path"
	"slices"
	"testing"
	"time"
)

// handWrittenExtras is what someone typed into book.json themselves. Every
// entry is a shape a decode into `any` would damage: a nested object, arrays
// whose order matters, a trailing-zero decimal, an integer past float64, a
// non-ASCII string, characters json.Marshal escapes by default, and the
// empty-ish values.
const handWrittenExtras = `{
  "series": "Genji Cycle",
  "series_index": 2,
  "douban_id": 1770782,
  "rating_precise": 4.500,
  "isbn_as_number": 9780142437148000000000000000001,
  "reading_notes": {
    "shelf": "top",
    "lent_to": ["Ann", "Bob"],
    "chapters": [{"n": 1, "done": true}, {"n": 2, "done": false}]
  },
  "note": "手改的註記",
  "link": "https://example.com/?a=1&b=2<3",
  "abandoned": null,
  "empty_object": {},
  "empty_array": []
}`

// decodeJSONObject returns a JSON object in the flat form a hand editor sees:
// every key still raw, so a comparison can be made on the bytes as typed.
func decodeJSONObject(t *testing.T, encoded []byte) map[string]json.RawMessage {
	t.Helper()

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &keys); err != nil {
		t.Fatalf("Failed to decode JSON object: %v", err)
	}
	return keys
}

func readBookJSONKeys(t *testing.T, metaPath string) map[string]json.RawMessage {
	t.Helper()

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", metaPath, err)
	}
	return decodeJSONObject(t, raw)
}

func compactJSON(t *testing.T, value json.RawMessage) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := json.Compact(&buf, value); err != nil {
		t.Fatalf("Failed to compact %s: %v", value, err)
	}
	return buf.Bytes()
}

// unknownKeysOf drops everything BookMeta itself owns, leaving the hand-added
// keys.
func unknownKeysOf(keys map[string]json.RawMessage) map[string]json.RawMessage {
	unknown := maps.Clone(keys)
	for key := range bookMetaKnownKeys {
		delete(unknown, key)
	}
	return unknown
}

// assertExtrasPreserved compares unknown keys byte for byte, after removing the
// whitespace between tokens: book.json is written as one indented document, so
// a nested value is re-indented to sit at its new depth. Everything that is not
// layout has to be identical — a lost decimal place, a widened integer, a
// changed escape, or a reordered array all fail here, and all of them would
// pass a comparison of decoded values.
func assertExtrasPreserved(t *testing.T, stage string, want, got map[string]json.RawMessage) {
	t.Helper()

	for _, key := range slices.Sorted(maps.Keys(want)) {
		gotValue, ok := got[key]
		if !ok {
			t.Errorf("%s: unknown key %q was dropped", stage, key)
			continue
		}
		if !bytes.Equal(compactJSON(t, gotValue), compactJSON(t, want[key])) {
			t.Errorf("%s: unknown key %q = %s, want %s", stage, key, gotValue, want[key])
		}
	}

	for _, key := range slices.Sorted(maps.Keys(got)) {
		if _, ok := want[key]; !ok {
			t.Errorf("%s: unexpected key %q = %s", stage, key, got[key])
		}
	}
}

// writeUnknownKeys merges hand-added keys into an existing book.json without
// going through BookMeta, the way a text editor would, escapes and all left
// alone, and ages the file so the shelf treats its cached copy as stale.
func writeUnknownKeys(t *testing.T, metaPath string, extras map[string]json.RawMessage) {
	t.Helper()

	keys := readBookJSONKeys(t, metaPath)
	maps.Copy(keys, extras)

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(keys); err != nil {
		t.Fatalf("Failed to encode hand-edited book.json: %v", err)
	}

	if err := os.WriteFile(metaPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("Failed to write hand-edited book.json: %v", err)
	}
	shiftModTime(t, metaPath, 2*time.Second)
}

func TestBookMetaUnmarshalCollectsUnknownKeys(t *testing.T) {
	fixture := `{
		"schema_version": 1,
		"id": "book-a82m",
		"title": "The Tale of Genji",
		"star": 5,
		"current_source": "20260315-a1",
		"series": "Genji Cycle",
		"reading_notes": {"shelf": "top", "lent_to": ["Ann", "Bob"]},
		"rating_precise": 4.500
	}`

	var meta BookMeta
	if err := json.Unmarshal([]byte(fixture), &meta); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if meta.Title != "The Tale of Genji" || meta.Star != 5 || meta.CurrentSource != "20260315-a1" {
		t.Fatalf("known fields decoded wrong: %+v", meta)
	}

	want := decodeJSONObject(t, []byte(`{
		"series": "Genji Cycle",
		"reading_notes": {"shelf": "top", "lent_to": ["Ann", "Bob"]},
		"rating_precise": 4.500
	}`))
	assertExtrasPreserved(t, "unmarshal", want, meta.Extra)
}

func TestBookMetaMarshalWritesUnknownKeysBack(t *testing.T) {
	before := BookMeta{
		SchemaVersion: BookMetaSchemaVersion,
		ID:            "book-a82m",
		Title:         "The Tale of Genji",
		Extra:         decodeJSONObject(t, []byte(handWrittenExtras)),
	}

	encoded, err := marshalBookMetaFile(&before)
	if err != nil {
		t.Fatalf("marshalBookMetaFile: %v", err)
	}

	var after BookMeta
	if err := json.Unmarshal(encoded, &after); err != nil {
		t.Fatalf("Unmarshal after marshal: %v", err)
	}

	if after.Title != before.Title || after.ID != before.ID {
		t.Errorf("known fields changed: %+v", after)
	}
	assertExtrasPreserved(t, "marshal", before.Extra, after.Extra)

	// Writing the same book twice must produce the same bytes: a book.json that
	// churns for no reason wakes up every backup and sync tool watching it.
	second, err := marshalBookMetaFile(&after)
	if err != nil {
		t.Fatalf("marshalBookMetaFile again: %v", err)
	}
	if !bytes.Equal(second, encoded) {
		t.Errorf("second write differs:\n%s\n---\n%s", second, encoded)
	}
}

func TestBookMetaMarshalWithoutUnknownKeysAddsNothing(t *testing.T) {
	meta := BookMeta{
		SchemaVersion: BookMetaSchemaVersion,
		ID:            "book-a82m",
		Title:         "The Tale of Genji",
	}

	withNilExtra, err := marshalBookMetaFile(&meta)
	if err != nil {
		t.Fatalf("marshalBookMetaFile: %v", err)
	}

	meta.Extra = map[string]json.RawMessage{}
	withEmptyExtra, err := marshalBookMetaFile(&meta)
	if err != nil {
		t.Fatalf("marshalBookMetaFile with an empty Extra: %v", err)
	}

	if !bytes.Equal(withEmptyExtra, withNilExtra) {
		t.Errorf("an empty Extra changed the file:\n%s\n---\n%s", withEmptyExtra, withNilExtra)
	}
	if unknown := unknownKeysOf(decodeJSONObject(t, withNilExtra)); len(unknown) != 0 {
		t.Errorf("a book with no hand-added fields gained keys: %v", slices.Sorted(maps.Keys(unknown)))
	}
}

func TestBookMetaMarshalPrefersTheKnownFieldOnAKeyClash(t *testing.T) {
	meta := BookMeta{
		SchemaVersion: BookMetaSchemaVersion,
		ID:            "book-a82m",
		Title:         "Known Title",
		Extra: map[string]json.RawMessage{
			"title":  json.RawMessage(`"Extra Title"`),
			"series": json.RawMessage(`"Genji Cycle"`),
		},
	}

	encoded, err := marshalBookMetaFile(&meta)
	if err != nil {
		t.Fatalf("marshalBookMetaFile: %v", err)
	}

	// A duplicated key is still valid JSON and the last one wins on decode, so
	// count the occurrences in the file rather than trusting a decode.
	if got := countTopLevelKey(t, encoded, "title"); got != 1 {
		t.Errorf("title appears %d times in the file, want 1:\n%s", got, encoded)
	}

	var after BookMeta
	if err := json.Unmarshal(encoded, &after); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if after.Title != "Known Title" {
		t.Errorf("title = %q, want the struct field to win", after.Title)
	}
	if value, ok := after.Extra["title"]; ok {
		t.Errorf("a known key came back as an unknown one: %s", value)
	}
}

// countTopLevelKey counts how many times a key appears in the outermost object
// of an encoded book.json.
func countTopLevelKey(t *testing.T, encoded []byte, key string) int {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	token, err := decoder.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		t.Fatalf("book.json does not start with an object, got %v", token)
	}

	count := 0
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			t.Fatalf("Token: %v", err)
		}
		if name, ok := nameToken.(string); ok && name == key {
			count++
		}

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			t.Fatalf("Decode value of %v: %v", nameToken, err)
		}
	}
	return count
}

// Unknown keys are a fact about the file, not public schema: everything that
// starts from a caller-held BookMeta — every API response, the exported cache —
// must be free of them.
func TestGetMetaKeepsUnknownKeysOutOfTheCallersCopy(t *testing.T) {
	book, _, tmpLib := newTestBook(t, "book-extra", "Extra Book")

	metaPath := path.Join(tmpLib, book.FolderPath(), BookMetaFile)
	writeUnknownKeys(t, metaPath, decodeJSONObject(t, []byte(handWrittenExtras)))

	reopened, err := openBook(book.root, newLoggerForTest(), book.FolderPath())
	if err != nil {
		t.Fatalf("openBook: %v", err)
	}

	if len(reopened.meta.Extra) == 0 {
		t.Fatal("the book read no unknown key at all, so this proves nothing")
	}
	if got := reopened.GetMeta().Extra; got != nil {
		t.Errorf("GetMeta exposed unknown keys: %v", got)
	}

	encoded, err := json.Marshal(reopened.GetMeta())
	if err != nil {
		t.Fatalf("Marshal the caller's copy: %v", err)
	}
	if unknown := unknownKeysOf(decodeJSONObject(t, encoded)); len(unknown) != 0 {
		t.Errorf("the caller's copy serializes unknown keys: %v", slices.Sorted(maps.Keys(unknown)))
	}
}

func TestExportedBookCacheOmitsUnknownKeys(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "lib")
	if err := os.CopyFS(libRoot, os.DirFS("testdata/lib")); err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	const bookID = "book-a82m"
	metaPath := path.Join(libRoot, booksFolder, "default", "test", bookID+".bookpkg", BookMetaFile)
	writeUnknownKeys(t, metaPath, decodeJSONObject(t, []byte(handWrittenExtras)))

	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		ScanInterval:      "0s",
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})
	if _, err := shelf.GetBook(bookID); err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read the exported cache: %v", err)
	}

	books := decodeJSONObject(t, decodeJSONObject(t, raw)["books"])
	entry, ok := books[bookID]
	if !ok {
		t.Fatalf("%s is not in the exported cache", bookID)
	}

	cachedMeta := decodeJSONObject(t, decodeJSONObject(t, entry)["meta"])
	if unknown := unknownKeysOf(cachedMeta); len(unknown) != 0 {
		t.Errorf("the exported cache carries unknown keys: %v", slices.Sorted(maps.Keys(unknown)))
	}
}

// A hand-edited book has to survive the writes PlainShelf performs on its own:
// rating it, giving it a cover, and moving it to another layer each rewrite
// book.json in full.
func TestUnknownBookJSONKeysSurviveEveryWritePath(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "lib")
	if err := os.CopyFS(tmpLib, os.DirFS("testdata/lib")); err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	const bookID = "book-a82m"
	metaPath := path.Join(tmpLib, booksFolder, "default", "test", bookID+".bookpkg", BookMetaFile)

	want := decodeJSONObject(t, []byte(handWrittenExtras))
	writeUnknownKeys(t, metaPath, want)

	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	book, err := shelf.GetBook(bookID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}

	meta := book.GetMeta()
	meta.Star = 4
	meta.Tags = []string{"classic"}
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	assertExtrasPreserved(t, "after SetMeta", want, unknownKeysOf(readBookJSONKeys(t, metaPath)))

	if err := book.SetCover([]byte("jpeg data"), ".jpg"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	assertExtrasPreserved(t, "after SetCover", want, unknownKeysOf(readBookJSONKeys(t, metaPath)))

	moved, err := shelf.MoveBook(bookID, Layers{"Fiction"})
	if err != nil {
		t.Fatalf("MoveBook: %v", err)
	}

	finalKeys := readBookJSONKeys(t, path.Join(tmpLib, moved.FolderPath(), BookMetaFile))
	assertExtrasPreserved(t, "after MoveBook", want, unknownKeysOf(finalKeys))

	// The known fields still took the edits, and the file is still a v1 book.
	for key, wantValue := range map[string]string{
		"star":           "4",
		"cover":          `"cover.jpg"`,
		"schema_version": "1",
	} {
		if got := string(finalKeys[key]); got != wantValue {
			t.Errorf("%s = %s, want %s", key, got, wantValue)
		}
	}
}
