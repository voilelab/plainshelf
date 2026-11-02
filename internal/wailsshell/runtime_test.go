package wailsshell

import (
	"strings"
	"testing"
)

const page = `<!doctype html><html><head lang="en"><title>x</title></head><body></body></html>`

func TestWithRuntimeOnSPARoute(t *testing.T) {
	got := string(WithRuntime("/books", []byte(page)))
	want := `<head lang="en">` + runtimeScripts + `<title>`
	if !strings.Contains(got, want) {
		t.Fatalf("runtime scripts not first in <head>: %s", got)
	}
	if strings.Index(got, "/wails/ipc.js") > strings.Index(got, "/wails/runtime.js") {
		t.Fatalf("ipc.js must precede runtime.js as Wails orders them: %s", got)
	}
}

func TestWithRuntimeLeavesWailsPaths(t *testing.T) {
	for _, path := range []string{"", "/", "/index.html", "/books/"} {
		if got := string(WithRuntime(path, []byte(page))); got != page {
			t.Fatalf("%q: Wails injects there itself, got %s", path, got)
		}
	}
}

func TestWithRuntimeWithoutHead(t *testing.T) {
	const bare = "<html><body></body></html>"
	if got := string(WithRuntime("/books", []byte(bare))); got != bare {
		t.Fatalf("page without <head> changed: %s", got)
	}
}
