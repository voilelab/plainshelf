package shelf

import (
	"bytes"
	"io"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
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
	_ = newTestShelf(t, &ShelfConf{LibRoot: tmpDir})

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
	source, err := createSource(rootFS, newLoggerForTest(), "test-source", "20260315-a4", srcFile)
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

	verified, err := source.VerifyContent()
	if err != nil {
		t.Fatalf("Failed to verify updated source content: %v", err)
	}
	if !verified {
		t.Fatal("Expected updated source content to match stored MD5 hash")
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
	_ = newTestShelf(t, &ShelfConf{LibRoot: tmpDir})

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
	source, err := createSource(rootFS, newLoggerForTest(), "test-source", "20260315-a3", srcFile)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	if source.ID() != "20260315-a3" {
		t.Errorf("Expected source ID '20260315-a3', got '%s'", source.ID())
	}

	meta := source.GetMeta()

	if meta.LineCount != 1 {
		t.Errorf("Expected line count 1, got %d", meta.LineCount)
	}

	if meta.CharCount != len(sourceContent) {
		t.Errorf("Expected character count %d, got %d", len(sourceContent), meta.CharCount)
	}
}

func TestSourceHashAndSplitConfigPersist(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})
	book, err := shelf.NewBook(Layers{"tests"}, "Source Metadata")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	source, err := book.NewSource(bytes.NewBufferString("original\n"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if source.FolderPath() == "" {
		t.Fatal("FolderPath returned an empty path")
	}

	if err := shelf.dbRoot.WriteFile(path.Join(source.FolderPath(), SourceFile), []byte("changed\n")); err != nil {
		t.Fatalf("replace source content: %v", err)
	}
	verified, err := source.VerifyContent()
	if err != nil {
		t.Fatalf("VerifyContent(changed): %v", err)
	}
	if verified {
		t.Fatal("VerifyContent accepted content with a stale hash")
	}
	if err := source.UpdateHash(); err != nil {
		t.Fatalf("UpdateHash: %v", err)
	}
	verified, err = source.VerifyContent()
	if err != nil {
		t.Fatalf("VerifyContent(updated): %v", err)
	}
	if !verified {
		t.Fatal("VerifyContent rejected content after UpdateHash")
	}

	wantConfig := SplitConfig{Type: SplitTypeBoundary, Boundaries: []int{10, 25, 50}}
	if err := source.UpdateSplitConfig(wantConfig); err != nil {
		t.Fatalf("UpdateSplitConfig: %v", err)
	}
	reopened, err := book.GetSource(source.ID())
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got := reopened.GetMeta().SplitConfig; !reflect.DeepEqual(got, wantConfig) {
		t.Fatalf("persisted split config = %#v, want %#v", got, wantConfig)
	}
}
