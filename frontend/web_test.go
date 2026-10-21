//go:build !(js && wasm) && !android

package frontend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWebEmbedIncludesUnderscorePrefixedChunks(t *testing.T) {
	source, err := os.ReadFile("web.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string][]byte{
		"go.mod":                 []byte("module embedcheck\n\ngo 1.18\n"),
		"web.go":                 source,
		"dist/index.html":        []byte("<script src=\"assets/_shared.js\"></script>"),
		"dist/assets/app.js":     []byte("import './_shared.js';"),
		"dist/assets/_shared.js": []byte("export const shared = true;"),
		"embed_test.go": []byte(`package frontend

import (
    "io/fs"
    "testing"
)

func TestSharedChunkIsEmbedded(t *testing.T) {
    got, err := fs.ReadFile(WebFS, "assets/_shared.js")
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "export const shared = true;" {
        t.Fatalf("shared chunk = %q", got)
    }
}
`),
	}
	for name, content := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the frontend embed with an underscore-prefixed chunk: %v\n%s", err, output)
	}
}
