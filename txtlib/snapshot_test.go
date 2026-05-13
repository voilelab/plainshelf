package txtlib

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestOpenSnapshot(t *testing.T) {
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "snapshots"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	snapshot, err := openSnapshot(rootFS, "20260315-a1")
	if err != nil {
		t.Fatalf("Failed to open snapshot: %v", err)
	}

	if snapshot.ID() != "20260315-a1" {
		t.Errorf("Expected snapshot ID '20260315-a1', got '%s'", snapshot.ID())
	}
}

func TestOpenSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "snapshots"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	snapshot, err := openSnapshot(rootFS, "20260315-a1")
	if err != nil {
		t.Fatalf("Failed to open snapshot: %v", err)
	}

	sourceFile, err := snapshot.OpenSource()
	if err != nil {
		t.Fatalf("Failed to open snapshot source: %v", err)
	}
	defer sourceFile.Close()

	readSrc, err := io.ReadAll(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}

	expectedSource := "This is the source text of the book snapshot.\n"
	if string(readSrc) != expectedSource {
		t.Errorf("Expected source content '%s', got '%s'", expectedSource, string(readSrc))
	}
}

func TestUpdateSource(t *testing.T) {
	tmpDir := path.Join(t.TempDir(), "txtlib_test")
	lib, err := OpenLocalLib(tmpDir)
	if err != nil {
		t.Fatalf("Failed to initialize Txtlib: %v", err)
	}
	defer lib.Close()

	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer tmpRoot.Close()

	sourceContent := "This is the source text of the book snapshot.\n"
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
	snapshot, err := createSnapshot(rootFS, "test-snapshot", "20260315-a4", srcFile)
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	newContent := "Updated source text for the book snapshot.\n"
	err = snapshot.UpdateContent(bytes.NewBufferString(newContent))
	if err != nil {
		t.Fatalf("Failed to update snapshot content: %v", err)
	}

	updatedSourceFile, err := snapshot.OpenSource()
	if err != nil {
		t.Fatalf("Failed to open updated snapshot source: %v", err)
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
	testdataRoot, err := os.OpenRoot(path.Join("testdata", "snapshots"))
	if err != nil {
		t.Fatalf("Failed to open testdata directory: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	snapshot, err := openSnapshot(rootFS, "20260315-a2")
	if err != nil {
		t.Fatalf("Failed to open snapshot: %v", err)
	}

	_, err = snapshot.OpenSource()
	if err == nil {
		t.Fatalf("Expected error when opening source for snapshot with missing source file, but got none")
	}
}

func TestCreateRootSnapshot(t *testing.T) {
	tmpDir := path.Join(t.TempDir(), "txtlib_test")
	lib, err := OpenLocalLib(tmpDir)
	if err != nil {
		t.Fatalf("Failed to initialize Txtlib: %v", err)
	}
	defer lib.Close()

	tmpRoot, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer tmpRoot.Close()

	sourceContent := "This is the source text of the book snapshot.\n"
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
	snapshot, err := createSnapshot(rootFS, "test-snapshot", "20260315-a3", srcFile)
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snapshot.ID() != "20260315-a3" {
		t.Errorf("Expected snapshot ID '20260315-a3', got '%s'", snapshot.ID())
	}

	meta := snapshot.GetMeta()

	if meta.LineCount != 1 {
		t.Errorf("Expected line count 1, got %d", meta.LineCount)
	}

	if meta.CharCount != len(sourceContent) {
		t.Errorf("Expected character count %d, got %d", len(sourceContent), meta.CharCount)
	}
}
