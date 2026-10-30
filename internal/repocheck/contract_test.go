package repocheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryAPITestHelperHasACaller(t *testing.T) {
	root := repoRoot(t)
	harness := filepath.Join(root, "server", "contract", "apitest")

	entries, err := os.ReadDir(harness)
	if err != nil {
		t.Fatalf("read %s: %v", harness, err)
	}

	fset := token.NewFileSet()
	declaredIn := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(harness, entry.Name())
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			declaredIn[fn.Name.Name] = entry.Name()
		}
	}
	if len(declaredIn) == 0 {
		t.Fatalf("found no exported functions in %s; has the harness moved?", harness)
	}

	callers := map[string]bool{}
	err = filepath.WalkDir(filepath.Join(root, "server", "contract"), func(pth string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(pth, ".go") {
			return err
		}
		file, parseErr := parser.ParseFile(fset, pth, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		name := filepath.Base(pth)
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if declaredIn[ident.Name] != name {
				callers[ident.Name] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the contract packages: %v", err)
	}

	for name, file := range declaredIn {
		if !callers[name] {
			t.Errorf("apitest/%s: %s has no caller in any contract package; delete it or use it", file, name)
		}
	}
}
