package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// The frontend renders folder icons from its own static imports, while the
// server decides which names a folder may carry. An icon only one side knows
// is either offered and refused, or accepted and drawn as the default.
func TestFrontendFolderIconsMatchTheServer(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "frontend", "src", "components", "icons", "folderIcons.ts"))
	if err != nil {
		t.Fatalf("read folderIcons.ts: %v", err)
	}

	var frontend []string
	for _, match := range regexp.MustCompile(`from '~icons/tabler/([a-z0-9-]+)'`).FindAllSubmatch(source, -1) {
		frontend = append(frontend, string(match[1]))
	}

	if server := shelf.FolderIcons(); !slices.Equal(frontend, server) {
		t.Fatalf("folderIcons.ts imports\n\t%q\nshelf.FolderIcons is\n\t%q", frontend, server)
	}
}
