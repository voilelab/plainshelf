package apitest

import (
	"os"
	"path/filepath"
	"testing"
)

// RepoRoot returns the repository root, found by walking up from the test's
// working directory until the root go.mod appears above the desktop module's.
// A contract test that reads a file the repository ships — a packaged config,
// say — addresses it from here rather than by counting ".." out of its own
// package, so moving a test between packages cannot silently break the path.
func RepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "desktop", "go.mod")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("locate the repository root above %q", dir)
		}
		dir = parent
	}
}
