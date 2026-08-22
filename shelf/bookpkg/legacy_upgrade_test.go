package bookpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// downgradeToLegacy rewrites a source's meta.json as one an older build wrote:
// no schema_version, no format, chapters carried by split_config. It returns a
// freshly opened handle, because the caller's handle still holds the old meta.
func downgradeToLegacy(t *testing.T, root fsutil.FS, source *Source, split string) *Source {
	t.Helper()

	metaPath := path.Join(source.FolderPath(), SourceMetaFile)
	legacy := `{"id":"` + source.ID() + `","created_at":"2026-01-01T00:00:00Z",` +
		`"comment":"legacy","split_config":` + split + `}`
	if err := root.WriteFile(metaPath, []byte(legacy)); err != nil {
		t.Fatalf("write legacy meta: %v", err)
	}

	legacySource, err := openSource(root, source.FolderPath())
	if err != nil {
		t.Fatalf("open legacy source: %v", err)
	}
	return legacySource
}

// readPersistedJSON decodes a book-relative JSON file as a bare map, so a test
// can assert on the keys actually on disk rather than on a struct's zero values.
func readPersistedJSON(t *testing.T, root fsutil.FS, filePath string) map[string]any {
	t.Helper()

	file, err := root.Open(filePath)
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

// newLegacyTestBook returns an empty book, the writable filesystem it lives on
// (for planting a legacy meta.json), and its on-disk library path, which the
// assertions need to stat files the FS only exposes relatively.
func newLegacyTestBook(t *testing.T, title string) (*Book, fsutil.FS, string) {
	t.Helper()

	return newTestBook(t, "legacy-book", title)
}

func TestUpgradeLegacyToSchemaV1RewritesContentAndStampsMeta(t *testing.T) {
	book, rootFS, libRoot := newLegacyTestBook(t, "Rewrite")
	source, err := book.NewSource(bytes.NewBufferString("a\nb\nc\nd"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	legacy := downgradeToLegacy(t, rootFS, source, `{"type":"line_count","line_count":2}`)

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

	persisted := readPersistedJSON(t, rootFS, path.Join(source.FolderPath(), SourceMetaFile))
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
	book, rootFS, libRoot := newLegacyTestBook(t, "Stamp only")
	source, err := book.NewSource(bytes.NewBufferString("untouched\ntext"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	legacy := downgradeToLegacy(t, rootFS, source, `{"type":""}`)

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

	persisted := readPersistedJSON(t, rootFS, path.Join(source.FolderPath(), SourceMetaFile))
	if persisted["format"] != BookFormatText {
		t.Errorf("format = %v, want %q", persisted["format"], BookFormatText)
	}
}

func TestUpgradeLegacyToSchemaV1Refusals(t *testing.T) {
	t.Run("a source that already owns its format", func(t *testing.T) {
		book, _, _ := newLegacyTestBook(t, "Already v1")
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
		book, rootFS, _ := newLegacyTestBook(t, "Bad format")
		source, err := book.NewSource(bytes.NewBufferString("text"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		legacy := downgradeToLegacy(t, rootFS, source, `{"type":""}`)

		for _, format := range []string{"", "epub", "TXT"} {
			if err := legacy.UpgradeLegacyToSchemaV1(format, nil); !errors.Is(err, ErrInvalidBookFormat) {
				t.Errorf("format %q: error = %v, want ErrInvalidBookFormat", format, err)
			}
		}
	})

	t.Run("a newer source schema, before the text is touched", func(t *testing.T) {
		book, rootFS, libRoot := newLegacyTestBook(t, "Future source")
		source, err := book.NewSource(bytes.NewBufferString("original"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		metaPath := path.Join(source.FolderPath(), SourceMetaFile)
		future := `{"schema_version":99,"id":"` + source.ID() +
			`","created_at":"2026-01-01T00:00:00Z","comment":"","split_config":{"type":""}}`
		if err := rootFS.WriteFile(metaPath, []byte(future)); err != nil {
			t.Fatalf("write future meta: %v", err)
		}
		futureSource, err := openSource(rootFS, source.FolderPath())
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
