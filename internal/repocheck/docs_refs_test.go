package repocheck

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type docRef struct {
	doc  string
	line int
	text string
}

var (
	inlineCode   = regexp.MustCompile("`([^`\n]+)`")
	goTestName   = regexp.MustCompile(`^Test[A-Z][A-Za-z0-9_]*$`)
	sourceFile   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*\.(go|ts|vue|mjs|js|sh|rb|kt|kts|java)$`)
	justRecipe   = regexp.MustCompile(`^just ([a-z][a-z0-9-]*)`)
	recipeHeader = regexp.MustCompile(`(?m)^@?([a-z][a-z0-9-]*)(?: [^:\n]*)?:(?:[^=]|$)`)
	goTestFunc   = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
)

var docRefExempt = map[string]string{
	"docs/development/mutation-testing.md parseMarkdownBlocks.ts": "former name, kept in the pilot's results table",
	"docs/development/mutation-testing.md sanitizeReaderHtml.ts":  "former name, kept in the pilot's results table",
	"docs/development/testing-levels.md vitest.config.ts":         "named to say it does not exist",
	"docs/development/android.md frontend/android/app/libs/":      "build output directory of just android-lib",
}

type repoIndex struct {
	root      string
	paths     map[string]bool
	basenames map[string]bool
	topLevel  map[string]bool
	testFuncs map[string]bool
	recipes   map[string]bool
}

func TestDocsReferencesExist(t *testing.T) {
	root := repoRoot(t)
	tracked := trackedFiles(t, root)
	repo := repoIndex{
		root:      root,
		paths:     map[string]bool{},
		basenames: map[string]bool{},
		topLevel:  map[string]bool{},
		testFuncs: map[string]bool{},
		recipes:   map[string]bool{},
	}
	for _, file := range tracked {
		repo.basenames[path.Base(file)] = true
		repo.topLevel[strings.SplitN(file, "/", 2)[0]] = true
		for p := file; p != "."; p = path.Dir(p) {
			repo.paths[p] = true
		}
		if strings.HasSuffix(file, "_test.go") {
			for _, m := range goTestFunc.FindAllSubmatch(readFile(t, root, file), -1) {
				repo.testFuncs[string(m[1])] = true
			}
		}
	}
	for _, m := range recipeHeader.FindAllSubmatch(readFile(t, root, "justfile"), -1) {
		repo.recipes[string(m[1])] = true
	}

	var missing []string
	for _, ref := range docInlineCode(t, root, tracked) {
		if _, ok := docRefExempt[ref.doc+" "+ref.text]; ok {
			continue
		}
		if problem := repo.check(ref.text); problem != "" {
			missing = append(missing, fmt.Sprintf("%s:%d: `%s` %s", ref.doc, ref.line, ref.text, problem))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("docs name things the repository does not have:\n\t%s", strings.Join(missing, "\n\t"))
	}
}

func (r repoIndex) check(text string) string {
	if m := justRecipe.FindStringSubmatch(text); m != nil {
		if !r.recipes[m[1]] {
			return "is not a justfile recipe"
		}
		return ""
	}
	if goTestName.MatchString(text) {
		if !r.testFuncs[text] {
			return "is not a Go test function in a tracked file"
		}
		return ""
	}
	if sourceFile.MatchString(text) {
		if !r.basenames[text] {
			return "matches no tracked file name (git add a new file)"
		}
		return ""
	}
	if p, ok := repoPathCandidate(text, r.topLevel); ok && !r.paths[p] && !gitIgnored(r.root, p) {
		return "is not a tracked path (git add a new file)"
	}
	return ""
}

func repoPathCandidate(text string, topLevel map[string]bool) (string, bool) {
	if strings.ContainsAny(text, " *<>{}$:?=,|()[]") || strings.Contains(text, "..") {
		return "", false
	}
	p := strings.TrimSuffix(strings.TrimPrefix(text, "./"), "/")
	first, _, nested := strings.Cut(p, "/")
	if !nested || !topLevel[first] {
		return "", false
	}
	return p, true
}

func docInlineCode(t *testing.T, root string, tracked []string) []docRef {
	t.Helper()
	var refs []docRef
	for _, doc := range tracked {
		if !strings.HasPrefix(doc, "docs/") || !strings.HasSuffix(doc, ".md") {
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(readFile(t, root, doc)))
		inFence := false
		for line := 1; scanner.Scan(); line++ {
			text := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(text), "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, m := range inlineCode.FindAllStringSubmatch(text, -1) {
				refs = append(refs, docRef{doc: doc, line: line, text: m[1]})
			}
		}
	}
	return refs
}

func gitIgnored(root, p string) bool {
	cmd := exec.Command("git", "check-ignore", "--no-index", p, p+"/")
	cmd.Dir = root
	out, _ := cmd.Output()
	return len(out) > 0
}

func trackedFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func readFile(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}
