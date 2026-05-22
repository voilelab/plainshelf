package shelf

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
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
	shelf, err := OpenLocalShelf(tmpDir)
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

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
	source, err := createSource(rootFS, "test-source", "20260315-a4", srcFile, logutil.NewDefaultLogger())
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	newContent := "Updated source text for the book source.\n"
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
	shelf, err := OpenLocalShelf(tmpDir)
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

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
	source, err := createSource(rootFS, "test-source", "20260315-a3", srcFile, logutil.NewDefaultLogger())
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
