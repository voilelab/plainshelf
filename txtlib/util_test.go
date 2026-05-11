package txtlib

import (
	"os"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestCreateTempDir(t *testing.T) {
	tmpDir := path.Join(os.TempDir(), "txtlib-test")
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer root.Close()

	err = root.MkdirAll("test/tmp", 0755)
	if err != nil {
		t.Fatalf("Failed to create test/tmp directory: %v", err)
	}

	tmpName, err := createTempDir(fsutil.NewRootFS(root), "test/tmp/a")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	if tmpName == "" {
		t.Fatalf("Temp dir name is empty")
	}

	// Check if the directory was actually created
	_, err = root.Open(tmpName)
	if err != nil {
		t.Fatalf("Failed to open created temp dir: %v", err)
	}
}

func TestValidateBCP47(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"en", true},
		{"zh-Hant", true},
		{"fr-CA", true},
		{"es-419", true},
		{"invalid-lang", false},
		{"123", false},
	}

	for _, test := range tests {
		result := validateBCP47(test.lang)
		if result != test.expected {
			t.Errorf("validateBCP47(%q) = %v; expected %v", test.lang, result, test.expected)
		}
	}
}
