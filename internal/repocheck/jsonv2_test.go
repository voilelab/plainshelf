package repocheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// encodingJSONPath is the v1 package. The v2 package is "encoding/json/v2" and
// is itself named json, so a converted call site reads the same and only the
// import line tells them apart — which is exactly why an editor's auto-import
// puts the wrong one back without anyone noticing.
const encodingJSONPath = "encoding/json"

// encodingJSONAllowlist names the Go files that may still import
// "encoding/json", with the reason each one is not converted yet. The
// conversion to encoding/json/v2 is docs/development/json-encoding.md; the
// point of the list is that it only shrinks, and that adding to it is a
// deliberate edit rather than an import an IDE completed.
//
// Paths are slash-separated and relative to the repository root.
var encodingJSONAllowlist = map[string]string{
	// shelf.json is the last read path on v1. Converting it adopts v2's strict
	// member matching for a file users hand-edit, which is a behavior decision
	// rather than an import swap, so it is PSW-99's rather than PSW-100's.
	"shelf/shelf_config.go": "strict shelf.json reads are PSW-99's decision to make",
}

// TestNoEncodingJSONV1Imports keeps "we use encoding/json/v2" a fact the build
// checks rather than a convention documented in a page nobody rereads. It walks
// every module in the repository - the root, desktop and reader - because a
// bare `go test ./...` in the root reaches none of the other two.
func TestNoEncodingJSONV1Imports(t *testing.T) {
	root := repoRoot(t)

	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, pth)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		if reason, ok := encodingJSONAllowlist[rel]; ok {
			if reason == "" {
				t.Errorf("%s: allowlisted without a reason", rel)
			}
			return nil
		}

		file, parseErr := parser.ParseFile(fset, pth, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil || path != encodingJSONPath {
				continue
			}
			t.Errorf("%s:%d: imports %q; use encoding/json/v2 and marshal through internal/jsonopt (see docs/development/json-encoding.md)",
				rel, fset.Position(spec.Pos()).Line, encodingJSONPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk the repository: %v", err)
	}
}

// TestEncodingJSONAllowlistIsCurrent stops an entry from outliving the file it
// excuses, which would silently re-open the exemption for a new file at that
// path, and stops one from outliving the import, so converting the file also
// closes its exemption.
func TestEncodingJSONAllowlistIsCurrent(t *testing.T) {
	root := repoRoot(t)

	fset := token.NewFileSet()
	for rel := range encodingJSONAllowlist {
		pth := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(pth); err != nil {
			t.Errorf("Allowlisted file %s is gone; drop the entry: %v", rel, err)
			continue
		}
		file, err := parser.ParseFile(fset, pth, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", rel, err)
			continue
		}
		if !importsEncodingJSONV1(file) {
			t.Errorf("Allowlisted file %s no longer imports %q; drop the entry", rel, encodingJSONPath)
		}
	}
}

func importsEncodingJSONV1(file *ast.File) bool {
	for _, spec := range file.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == encodingJSONPath {
			return true
		}
	}
	return false
}
