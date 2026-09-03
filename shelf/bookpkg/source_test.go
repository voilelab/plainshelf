package bookpkg

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
)

func TestOpenSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "sources"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	source, err := openSource(rootFS, "20260315-a1")
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	if source.ID() != "20260315-a1" {
		t.Errorf("Expected source ID '20260315-a1', got '%s'", source.ID())
	}
}

func TestOpenFileOfSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "sources"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	source, err := openSource(rootFS, "20260315-a1")
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	sourceFile, err := source.Open()
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	readSrc, err := io.ReadAll(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}

	expectedSource := "This is the source text of the book source.\n"
	if string(readSrc) != expectedSource {
		t.Errorf("Expected source content '%s', got '%s'", expectedSource, string(readSrc))
	}
}

func TestUpdateSource(t *testing.T) {
	tmpDir := path.Join(t.TempDir(), "shelf_test")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer tmpRoot.Close()

	sourceContent := "This is the source text of the book source.\n"
	sourceFilePath := path.Join(t.TempDir(), "temp_source.txt")
	err = os.WriteFile(sourceFilePath, []byte(sourceContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temporary source file: %v", err)
	}

	srcFile, err := os.Open(sourceFilePath)
	if err != nil {
		t.Fatalf("Failed to open temporary source file: %v", err)
	}
	defer srcFile.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	source, err := createSource(rootFS, newLoggerForTest(), "test-source", "20260315-a4", srcFile, BookFormatText, "")
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	originalMeta := *source.GetMeta()

	newContent := "Updated source text for the book source.\nSecond line\n三"
	err = source.UpdateContent(bytes.NewBufferString(newContent))
	if err != nil {
		t.Fatalf("Failed to update source content: %v", err)
	}

	updatedSourceFile, err := source.Open()
	if err != nil {
		t.Fatalf("Failed to open updated source source: %v", err)
	}
	defer updatedSourceFile.Close()

	updatedSrc, err := io.ReadAll(updatedSourceFile)
	if err != nil {
		t.Fatalf("Failed to read updated source file: %v", err)
	}

	if string(updatedSrc) != newContent {
		t.Errorf("Expected updated source content '%s', got '%s'", newContent, string(updatedSrc))
	}

	updatedMeta := source.GetMeta()
	if updatedMeta.MD5Hash == originalMeta.MD5Hash {
		t.Error("Expected MD5 hash to change after source content update")
	}
	if updatedMeta.LineCount != 3 {
		t.Errorf("Expected line count 3 after content update, got %d", updatedMeta.LineCount)
	}
	if updatedMeta.LineCount == originalMeta.LineCount {
		t.Error("Expected line count to change after source content update")
	}
	if updatedMeta.CharCount != len([]rune(newContent)) {
		t.Errorf("Expected character count %d after content update, got %d", len([]rune(newContent)), updatedMeta.CharCount)
	}
	if updatedMeta.CharCount == originalMeta.CharCount {
		t.Error("Expected character count to change after source content update")
	}

	if got := contentMD5(t, source); got != updatedMeta.MD5Hash {
		t.Fatalf("meta md5_hash = %q, want the updated content's %q", updatedMeta.MD5Hash, got)
	}
}

func TestOpenSourceInvalid(t *testing.T) {
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "sources"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	source, err := openSource(rootFS, "20260315-a2")
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	_, err = source.Open()
	if err == nil {
		t.Fatalf("Expected error when opening source for source with missing source file, but got none")
	}
}

func TestCreateRootSource(t *testing.T) {
	tmpDir := path.Join(t.TempDir(), "shelf_test")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer tmpRoot.Close()

	sourceContent := "This is the source text of the book source.\n"
	sourceFilePath := path.Join(t.TempDir(), "temp_source.txt")
	err = os.WriteFile(sourceFilePath, []byte(sourceContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temporary source file: %v", err)
	}

	srcFile, err := os.Open(sourceFilePath)
	if err != nil {
		t.Fatalf("Failed to open temporary source file: %v", err)
	}
	defer srcFile.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	source, err := createSource(rootFS, newLoggerForTest(), "test-source", "20260315-a3", srcFile, BookFormatText, "")
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	if source.ID() != "20260315-a3" {
		t.Errorf("Expected source ID '20260315-a3', got '%s'", source.ID())
	}

	meta := source.GetMeta()
	if meta.SchemaVersion != SourceMetaSchemaVersion || meta.Format != BookFormatText {
		t.Fatalf("new source meta = %+v, want schema %d txt", meta, SourceMetaSchemaVersion)
	}

	if meta.LineCount != 1 {
		t.Errorf("Expected line count 1, got %d", meta.LineCount)
	}

	if meta.CharCount != len(sourceContent) {
		t.Errorf("Expected character count %d, got %d", len(sourceContent), meta.CharCount)
	}
}

func TestLegacySourceSaveDoesNotUpgradeFormatOwnership(t *testing.T) {
	book, rootFS, _ := newTestBook(t, "legacy-book", "Legacy")
	source, err := book.NewSource(bytes.NewBufferString("before"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	metaPath := path.Join(source.FolderPath(), SourceMetaFile)
	// A pre-schema-version source carries whatever keys an older build wrote,
	// including fields this build no longer models. Opening it decodes only the
	// known fields, so a rewrite persists those and does not resurrect the
	// unknown ones, nor upgrade the source by adding format/schema_version.
	legacy := `{"id":"` + source.ID() + `","created_at":"2026-01-01T00:00:00Z","comment":"legacy","legacy_extra":"ignored"}`
	if err := rootFS.WriteFile(metaPath, []byte(legacy)); err != nil {
		t.Fatalf("write legacy meta: %v", err)
	}

	legacySource, err := openSource(rootFS, source.FolderPath())
	if err != nil {
		t.Fatalf("open legacy source: %v", err)
	}
	if err := legacySource.UpdateContent(bytes.NewBufferString("after")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}
	metaFile, err := rootFS.Open(metaPath)
	if err != nil {
		t.Fatalf("open legacy meta: %v", err)
	}
	raw, err := io.ReadAll(metaFile)
	metaFile.Close()
	if err != nil {
		t.Fatalf("read legacy meta: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode persisted meta: %v", err)
	}
	if _, ok := persisted["format"]; ok {
		t.Fatalf("ordinary save added format to legacy source: %s", raw)
	}
	if _, ok := persisted["schema_version"]; ok {
		t.Fatalf("ordinary save added schema_version to legacy source: %s", raw)
	}
	if _, ok := persisted["legacy_extra"]; ok {
		t.Fatalf("ordinary save preserved an unknown legacy field: %s", raw)
	}
	if persisted["comment"] != "legacy" {
		t.Fatalf("ordinary save changed a known field: %s", raw)
	}
}

func TestNewerSourceSchemaIsReadableButNotWritable(t *testing.T) {
	book, rootFS, _ := newTestBook(t, "future-book", "Future source")
	source, err := book.NewSource(bytes.NewBufferString("original"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	metaPath := path.Join(source.FolderPath(), SourceMetaFile)
	future := `{"schema_version":99,"id":"` + source.ID() + `","created_at":"2026-01-01T00:00:00Z","comment":"future","format":"md","future_key":true}`
	if err := rootFS.WriteFile(metaPath, []byte(future)); err != nil {
		t.Fatalf("write future meta: %v", err)
	}
	futureSource, err := openSource(rootFS, source.FolderPath())
	if err != nil {
		t.Fatalf("future source should remain readable: %v", err)
	}
	if err := futureSource.UpdateContent(bytes.NewBufferString("clobbered")); !errors.Is(err, ErrUnsupportedSourceSchemaVersion) {
		t.Fatalf("UpdateContent error = %v, want ErrUnsupportedSourceSchemaVersion", err)
	}
	sourceFile, err := rootFS.Open(path.Join(source.FolderPath(), SourceFile))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	raw, err := io.ReadAll(sourceFile)
	sourceFile.Close()
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(raw) != "original" {
		t.Fatalf("refused write changed source to %q", raw)
	}
}

func TestRepairContentHashRewritesAStaleHash(t *testing.T) {
	book, rootFS, _ := newTestBook(t, "hash-book", "Source Metadata")
	source, err := book.NewSource(bytes.NewBufferString("original\n"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if source.FolderPath() == "" {
		t.Fatal("FolderPath returned an empty path")
	}
	stored := source.GetMeta().MD5Hash
	if stored == "" {
		t.Fatal("a new source was written without an md5_hash")
	}

	// Edited outside PlainShelf: the content moves, meta.json does not.
	if err := rootFS.WriteFile(path.Join(source.FolderPath(), SourceFile), []byte("changed\n")); err != nil {
		t.Fatalf("replace source content: %v", err)
	}
	edited := contentMD5(t, source)
	if edited == stored {
		t.Fatal("the fixture did not actually change the content")
	}
	if got := source.GetMeta().MD5Hash; got != stored {
		t.Fatalf("md5_hash = %q before any repair, want the stale %q", got, stored)
	}

	repaired, err := source.RepairContentHash(edited)
	if err != nil {
		t.Fatalf("RepairContentHash: %v", err)
	}
	if !repaired {
		t.Fatal("RepairContentHash reported no write for a hash that disagreed")
	}
	if got := source.GetMeta().MD5Hash; got != edited {
		t.Fatalf("md5_hash = %q after repair, want %q", got, edited)
	}

	// The write reached meta.json, not just the in-memory source.
	reopened, err := book.GetSource(source.GetMeta().ID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got := reopened.GetMeta().MD5Hash; got != edited {
		t.Fatalf("md5_hash = %q after reopening, want the persisted %q", got, edited)
	}

	// Agreeing hashes are a no-op rather than a second write.
	again, err := reopened.RepairContentHash(edited)
	if err != nil {
		t.Fatalf("RepairContentHash (agreeing): %v", err)
	}
	if again {
		t.Error("RepairContentHash rewrote meta.json for a hash that already agreed")
	}
}

// contentMD5 is the hash meta.json should be carrying for a source: the tests
// that used to call Source.VerifyContent compare against this instead.
func contentMD5(t *testing.T, source *Source) string {
	t.Helper()
	f, err := source.Open()
	if err != nil {
		t.Fatalf("Open source content: %v", err)
	}
	defer f.Close()
	sum, err := hashutil.MD5Hash(f)
	if err != nil {
		t.Fatalf("MD5Hash: %v", err)
	}
	return sum
}

// A source's meta.json is written by the same encoder as book.json, and its
// comment is free-form text a reader typed. json/v2 leaves &, < and > alone
// where v1 wrote \u0026, \u003c and \u003e into a file whose point is that a
// text editor shows what you typed.
func TestCreateSourceWritesCommentLiterally(t *testing.T) {
	const comment = `imported from "A & B" <draft>`

	tmpDir := t.TempDir()
	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	source, err := createSource(rootFS, newLoggerForTest(), "commented-source", "20260315-a5",
		bytes.NewBufferString("body"), BookFormatText, comment)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	raw, err := os.ReadFile(path.Join(tmpDir, "commented-source", SourceMetaFile))
	if err != nil {
		t.Fatalf("Failed to read meta.json: %v", err)
	}
	if !strings.Contains(string(raw), `"comment": "imported from \"A & B\" <draft>"`) {
		t.Errorf("meta.json does not carry the comment literally:\n%s", raw)
	}
	if got := source.GetMeta().Comment; got != comment {
		t.Errorf("Comment did not round-trip: got %q, want %q", got, comment)
	}
}
