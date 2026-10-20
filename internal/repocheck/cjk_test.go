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
	"unicode"
)

var cjkStringAllowlist = map[string]string{
	"internal/util/detect_lang.go":    "Han character sets used to tell zh-Hant from zh-Hans",
	"internal/testutil/epub.go":       "CJK EPUB fixture content for tests",
	"server/contract/apitest/book.go": "non-ASCII tail every imported fixture carries, so the decoding path is exercised",
}

var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"testdata":     true,
}

func TestNoCJKStringLiteralsInNonTestGo(t *testing.T) {
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		if reason, ok := cjkStringAllowlist[rel]; ok {
			if reason == "" {
				t.Errorf("%s: allowlisted without a reason", rel)
			}
			return nil
		}

		file, parseErr := parser.ParseFile(fset, pth, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil {
				value = lit.Value
			}
			if r, found := firstCJK(value); found {
				t.Errorf("%s:%d: CJK character %q in a string literal; write it in English or add the file to cjkStringAllowlist with a reason",
					rel, fset.Position(lit.Pos()).Line, r)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk the repository: %v", err)
	}
}

func TestCJKStringAllowlistIsCurrent(t *testing.T) {
	root := repoRoot(t)
	for rel := range cjkStringAllowlist {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("Allowlisted file %s is gone; drop the entry: %v", rel, err)
		}
	}
}

func firstCJK(s string) (rune, bool) {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) ||
			unicode.Is(unicode.Hangul, r) {
			return r, true
		}
	}
	return 0, false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "desktop", "go.mod")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Failed to locate the repository root above %q", dir)
		}
		dir = parent
	}
}
