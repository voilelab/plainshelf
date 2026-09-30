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

// The API contract harness is package server/contract/apitest rather than a set
// of _test.go files, so the contract packages can share it. That costs one
// check: staticcheck's `unused` does not report an exported identifier, so a
// harness function every contract package has stopped calling would sit there
// indefinitely — which is not true of the unexported helpers it replaced.
//
// This restores the floor for functions, which is where the helpers are. Types,
// constants and methods are left out on purpose: a type is used through the
// signatures it appears in and a method may exist to satisfy an interface, so
// counting references to those would need an allowlist, and an allowlist is how
// a check stops being believed.
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
			// A reference inside the file that declares the function is the
			// declaration itself or a call the harness makes to its own helper;
			// neither proves a contract test still needs it.
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
