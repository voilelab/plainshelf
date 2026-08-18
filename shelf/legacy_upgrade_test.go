package shelf

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"testing"
)

// downgradeToLegacy rewrites a source's meta.json as one an older build wrote:
// no schema_version, no format, chapters carried by split_config. It returns a
// freshly opened handle, because the caller's handle still holds the old meta.
func downgradeToLegacy(t *testing.T, testShelf *Shelf, source *Source, split string) *Source {
	t.Helper()

	metaPath := path.Join(source.FolderPath(), SourceMetaFile)
	legacy := `{"id":"` + source.ID() + `","created_at":"2026-01-01T00:00:00Z",` +
		`"comment":"legacy","split_config":` + split + `}`
	if err := testShelf.dbRoot.WriteFile(metaPath, []byte(legacy)); err != nil {
		t.Fatalf("write legacy meta: %v", err)
	}

	legacySource, err := openSource(testShelf.dbRoot, source.FolderPath())
	if err != nil {
		t.Fatalf("open legacy source: %v", err)
	}
	return legacySource
}

// readPersistedJSON decodes a shelf-relative JSON file as a bare map, so a test
// can assert on the keys actually on disk rather than on a struct's zero values.
func readPersistedJSON(t *testing.T, testShelf *Shelf, filePath string) map[string]any {
	t.Helper()

	file, err := testShelf.dbRoot.Open(filePath)
	if err != nil {
		t.Fatalf("open %s: %v", filePath, err)
	}
	raw, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}

	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode %s: %v", filePath, err)
	}
	return persisted
}

// newLegacyTestBook returns the shelf, an empty book, and the shelf's on-disk
// root, which the assertions need to stat files the shelf FS only exposes
// relatively.
func newLegacyTestBook(t *testing.T, title string) (*Shelf, *Book, string) {
	t.Helper()

	libRoot := t.TempDir()
	testShelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	book, err := testShelf.NewBook(nil, title)
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	return testShelf, book, libRoot
}

func TestUpgradeLegacyToSchemaV1RewritesContentAndStampsMeta(t *testing.T) {
	testShelf, book, libRoot := newLegacyTestBook(t, "Rewrite")
	source, err := book.NewSource(bytes.NewBufferString("a\nb\nc\nd"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	legacy := downgradeToLegacy(t, testShelf, source, `{"type":"line_count","line_count":2}`)

	upgraded := "## Part 1\n\na\nb\n## Part 2\n\nc\nd"
	if err := legacy.UpgradeLegacyToSchemaV1(BookFormatMarkdown, bytes.NewBufferString(upgraded)); err != nil {
		t.Fatalf("UpgradeLegacyToSchemaV1: %v", err)
	}

	content, err := os.ReadFile(path.Join(libRoot, source.FolderPath(), SourceFile))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(content) != upgraded {
		t.Errorf("source.txt = %q, want %q", content, upgraded)
	}

	persisted := readPersistedJSON(t, testShelf, path.Join(source.FolderPath(), SourceMetaFile))
	if persisted["schema_version"] != float64(SourceMetaSchemaVersion) {
		t.Errorf("schema_version = %v, want %d", persisted["schema_version"], SourceMetaSchemaVersion)
	}
	if persisted["format"] != BookFormatMarkdown {
		t.Errorf("format = %v, want %q", persisted["format"], BookFormatMarkdown)
	}
	split, _ := persisted["split_config"].(map[string]any)
	if split["type"] != "" {
		t.Errorf("split_config.type = %v, want the ignored empty type", split["type"])
	}
	if _, ok := split["line_count"]; ok {
		t.Errorf("split_config kept its legacy line_count: %v", split)
	}

	// The content metrics must describe the new bytes, not the old ones.
	meta := legacy.GetMeta()
	if meta.CharCount != len([]rune(upgraded)) {
		t.Errorf("char_count = %d, want %d", meta.CharCount, len([]rune(upgraded)))
	}
	if ok, err := legacy.VerifyContent(); err != nil || !ok {
		t.Errorf("VerifyContent() = %v, %v; want the hash to match the new content", ok, err)
	}
}

func TestUpgradeLegacyToSchemaV1WithoutContentLeavesTheTextAlone(t *testing.T) {
	testShelf, book, libRoot := newLegacyTestBook(t, "Stamp only")
	source, err := book.NewSource(bytes.NewBufferString("untouched\ntext"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	legacy := downgradeToLegacy(t, testShelf, source, `{"type":""}`)

	sourcePath := path.Join(libRoot, source.FolderPath(), SourceFile)
	before, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}

	if err := legacy.UpgradeLegacyToSchemaV1(BookFormatText, nil); err != nil {
		t.Fatalf("UpgradeLegacyToSchemaV1: %v", err)
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(content) != "untouched\ntext" {
		t.Errorf("source.txt = %q, want it unchanged", content)
	}
	after, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("source.txt was rewritten: mtime %v became %v", before.ModTime(), after.ModTime())
	}

	persisted := readPersistedJSON(t, testShelf, path.Join(source.FolderPath(), SourceMetaFile))
	if persisted["format"] != BookFormatText {
		t.Errorf("format = %v, want %q", persisted["format"], BookFormatText)
	}
}

func TestUpgradeLegacyToSchemaV1Refusals(t *testing.T) {
	t.Run("a source that already owns its format", func(t *testing.T) {
		_, book, _ := newLegacyTestBook(t, "Already v1")
		source, err := book.NewSource(bytes.NewBufferString("text"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		err = source.UpgradeLegacyToSchemaV1(BookFormatMarkdown, nil)
		if !errors.Is(err, ErrSourceNotLegacy) {
			t.Fatalf("error = %v, want ErrSourceNotLegacy", err)
		}
	})

	t.Run("an unknown format", func(t *testing.T) {
		testShelf, book, _ := newLegacyTestBook(t, "Bad format")
		source, err := book.NewSource(bytes.NewBufferString("text"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		legacy := downgradeToLegacy(t, testShelf, source, `{"type":""}`)

		for _, format := range []string{"", "epub", "TXT"} {
			if err := legacy.UpgradeLegacyToSchemaV1(format, nil); !errors.Is(err, ErrInvalidBookFormat) {
				t.Errorf("format %q: error = %v, want ErrInvalidBookFormat", format, err)
			}
		}
	})

	t.Run("a newer source schema, before the text is touched", func(t *testing.T) {
		testShelf, book, libRoot := newLegacyTestBook(t, "Future source")
		source, err := book.NewSource(bytes.NewBufferString("original"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		metaPath := path.Join(source.FolderPath(), SourceMetaFile)
		future := `{"schema_version":99,"id":"` + source.ID() +
			`","created_at":"2026-01-01T00:00:00Z","comment":"","split_config":{"type":""}}`
		if err := testShelf.dbRoot.WriteFile(metaPath, []byte(future)); err != nil {
			t.Fatalf("write future meta: %v", err)
		}
		futureSource, err := openSource(testShelf.dbRoot, source.FolderPath())
		if err != nil {
			t.Fatalf("open future source: %v", err)
		}

		err = futureSource.UpgradeLegacyToSchemaV1(BookFormatMarkdown, bytes.NewBufferString("replaced"))
		if !errors.Is(err, ErrUnsupportedSourceSchemaVersion) {
			t.Fatalf("error = %v, want ErrUnsupportedSourceSchemaVersion", err)
		}

		content, err := os.ReadFile(path.Join(libRoot, source.FolderPath(), SourceFile))
		if err != nil {
			t.Fatalf("read source: %v", err)
		}
		if string(content) != "original" {
			t.Errorf("source.txt = %q, want the refusal to have come before the write", content)
		}
	})
}

// legacyBookWithSnapshot builds a book whose current source is legacy and whose
// book.json therefore carries a legacy_source_formats snapshot.
func legacyBookWithSnapshot(t *testing.T) (*Shelf, *Book, *Source, string) {
	t.Helper()

	testShelf, book, libRoot := newLegacyTestBook(t, "Snapshot")
	first, err := book.NewSource(bytes.NewBufferString("first"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	// The snapshot is taken as a legacy source is deactivated, so it has to be
	// the current one before the downgrade.
	if err := book.SetCurrentSource(first.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}
	legacyFirst := downgradeToLegacy(t, testShelf, first, `{"type":""}`)

	// Activating a second, schema-versioned source is what makes the shelf
	// snapshot the outgoing legacy source's effective format.
	second, err := book.NewSourceWithOptions(bytes.NewBufferString("## Second"),
		NewSourceOptions{Format: BookFormatMarkdown})
	if err != nil {
		t.Fatalf("NewSourceWithOptions: %v", err)
	}
	if err := book.SetCurrentSource(second.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}
	if len(book.GetMeta().LegacySourceFormats) == 0 {
		t.Fatalf("expected a legacy_source_formats snapshot to exist")
	}
	return testShelf, book, legacyFirst, libRoot
}

func TestClearLegacySourceFormats(t *testing.T) {
	testShelf, book, legacy, _ := legacyBookWithSnapshot(t)

	if err := book.ClearLegacySourceFormats(); !errors.Is(err, ErrBookHasLegacySources) {
		t.Fatalf("error = %v, want ErrBookHasLegacySources while a legacy source remains", err)
	}
	if len(book.GetMeta().LegacySourceFormats) == 0 {
		t.Fatalf("a refused clear must leave the snapshots in place")
	}

	if err := legacy.UpgradeLegacyToSchemaV1(BookFormatText, nil); err != nil {
		t.Fatalf("UpgradeLegacyToSchemaV1: %v", err)
	}
	if err := book.ClearLegacySourceFormats(); err != nil {
		t.Fatalf("ClearLegacySourceFormats: %v", err)
	}

	persisted := readPersistedJSON(t, testShelf, path.Join(book.FolderPath(), BookMetaFile))
	if _, ok := persisted["legacy_source_formats"]; ok {
		t.Errorf("book.json still carries legacy_source_formats: %v", persisted["legacy_source_formats"])
	}
	if len(book.GetMeta().LegacySourceFormats) != 0 {
		t.Errorf("in-memory meta still carries snapshots")
	}
}

func TestClearLegacySourceFormatsOnAnEmptyMapDoesNotRewriteBookJSON(t *testing.T) {
	_, book, libRoot := newLegacyTestBook(t, "No snapshots")
	metaPath := path.Join(libRoot, book.FolderPath(), BookMetaFile)

	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	beforeStat, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("stat book.json: %v", err)
	}

	if err := book.ClearLegacySourceFormats(); err != nil {
		t.Fatalf("ClearLegacySourceFormats: %v", err)
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	afterStat, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("stat book.json: %v", err)
	}
	if !bytes.Equal(before, after) || !beforeStat.ModTime().Equal(afterStat.ModTime()) {
		t.Errorf("book.json was rewritten with nothing to change")
	}
}

// TestSetMetaStillCannotClearLegacySourceFormats guards the boundary the new
// affordance opens: an ordinary metadata update must still be unable to erase
// the snapshots.
func TestSetMetaStillCannotClearLegacySourceFormats(t *testing.T) {
	_, book, _, _ := legacyBookWithSnapshot(t)

	meta := book.GetMeta()
	meta.LegacySourceFormats = nil
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	if len(book.GetMeta().LegacySourceFormats) == 0 {
		t.Errorf("SetMeta erased the compatibility snapshots")
	}
}
