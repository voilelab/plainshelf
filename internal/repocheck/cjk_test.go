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

// cjkStringAllowlist names the non-test Go files whose string literals may
// contain CJK characters, with the reason each one needs them. A file not
// listed here is a bug: PlainShelf's UI is translated in the frontend, and the
// server has no locale to translate against, so a CJK literal reaching a user —
// in a file the shelf writes, an API response, or a log line — is text an
// English reader cannot act on. See shelf.CurrentSourceHintTemplate for the
// case that motivated this check.
//
// Paths are slash-separated and relative to the repository root.
var cjkStringAllowlist = map[string]string{
	// Language detection compares against sets of Han characters; the data is
	// the point and is never shown to a user.
	"internal/util/detect_lang.go": "Han character sets used to tell zh-Hant from zh-Hans",
	// Test fixtures that happen to live outside a _test.go file so several
	// packages can share them.
	"internal/testutil/epub.go": "CJK EPUB fixture content for tests",
}

// skippedDirs are directories with no first-party Go source worth parsing.
var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"testdata":     true,
}

// TestNoCJKStringLiteralsInNonTestGo keeps hardcoded CJK out of shipped Go
// strings. Comments are exempt on purpose: they address contributors, who work
// in Chinese, while strings can reach users, who may not.
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

// TestCJKStringAllowlistIsCurrent stops the allowlist from outliving the files
// it excuses, which would quietly re-open the hole for a new file at that path.
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

// repoRoot returns the repository root, found by walking up from the test's
// working directory until a go.mod appears next to the .claude rules directory.
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
