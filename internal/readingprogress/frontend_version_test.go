package readingprogress

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

func TestDocumentVersionMatchesTheFrontend(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "storage", "readingProgress", "document.ts"))
	if err != nil {
		t.Fatalf("read document.ts: %v", err)
	}
	match := regexp.MustCompile(`export const READING_PROGRESS_DOCUMENT_VERSION = (\d+);`).FindSubmatch(source)
	if match == nil {
		t.Fatal("document.ts no longer declares READING_PROGRESS_DOCUMENT_VERSION")
	}
	frontend, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("parse READING_PROGRESS_DOCUMENT_VERSION: %v", err)
	}
	if frontend != DocumentVersion {
		t.Errorf("frontend READING_PROGRESS_DOCUMENT_VERSION is %d, DocumentVersion is %d", frontend, DocumentVersion)
	}
}
