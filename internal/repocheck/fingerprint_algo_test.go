package repocheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const fingerprintAlgoOwner = "server/task/task_fingerprint_sources.go"

func TestOnlyTheFingerprintTaskBuildsAFingerprintAlgo(t *testing.T) {
	root := repoRoot(t)

	fset := token.NewFileSet()
	found := false
	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, pth)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, pth, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !namesFingerprintAlgo(lit.Type) {
				return true
			}
			if rel == fingerprintAlgoOwner {
				found = true
				return true
			}
			t.Errorf("%s: builds a FingerprintAlgo; call task.FingerprintAlgo() so every reader opens the cache with the same value", fset.Position(lit.Pos()))
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository: %v", err)
	}
	if !found {
		t.Errorf("%s no longer builds the FingerprintAlgo; move fingerprintAlgoOwner with it", fingerprintAlgoOwner)
	}
}

func namesFingerprintAlgo(expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.Ident:
		return typ.Name == "FingerprintAlgo"
	case *ast.SelectorExpr:
		return typ.Sel.Name == "FingerprintAlgo"
	}
	return false
}
