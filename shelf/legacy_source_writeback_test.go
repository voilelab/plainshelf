package shelf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestLegacySourceWriteBackDropsRetiredFields pins the write path for a legacy
// source that still carries the retired chapter-split field on disk (the
// legacy-schema conformance fixture's meta.json holds a real one). Reading it is
// covered by the conformance suite — the field is an unknown key that decodes
// into nothing. This is the other half: an operation that rewrites meta.json
// must make a definite choice, not leave the field in an ambiguous half-state.
//
// The decided behavior is that the rewrite persists only the fields this build
// models and drops every unknown key, while leaving the known metadata intact.
// A source opened here is schema-version 0, so writes are allowed (a future
// schema would refuse them and never reach this path).
func TestLegacySourceWriteBackDropsRetiredFields(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "cases", "legacy-schema", "shelf")
	libRoot := filepath.Join(t.TempDir(), "shelf")
	if err := os.CopyFS(libRoot, os.DirFS(fixture)); err != nil {
		t.Fatalf("copy legacy fixture: %v", err)
	}

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, err := s.GetBook("3f9a2c71")
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if err := source.UpdateComment("touched"); err != nil {
		t.Fatalf("UpdateComment: %v", err)
	}

	metaPath := filepath.Join(libRoot, "books", "old-tales.bookpkg",
		"sources", "20250102-030405", SourceMetaFile)
	raw, err := os.ReadFile(metaPath) //nolint:gosec // fixed test fixture path
	if err != nil {
		t.Fatalf("read rewritten meta: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode rewritten meta: %v", err)
	}

	keys := make([]string, 0, len(persisted))
	for key := range persisted {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	// Only the modeled fields survive: the new comment plus the identity fields
	// the legacy source already had. The retired chapter-split field, and any
	// other unknown key, is gone rather than carried forward.
	if want := []string{"comment", "created_at", "id"}; !slices.Equal(keys, want) {
		t.Fatalf("rewritten meta keys = %v, want %v; body: %s", keys, want, raw)
	}
	if persisted["comment"] != "touched" {
		t.Fatalf("comment = %v, want the updated value; body: %s", persisted["comment"], raw)
	}
	if persisted["id"] != "20250102-030405" {
		t.Fatalf("id changed to %v; body: %s", persisted["id"], raw)
	}
	if persisted["created_at"] != "2025-01-02T03:04:05Z" {
		t.Fatalf("created_at changed to %v; body: %s", persisted["created_at"], raw)
	}
}
