package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/epub"
)

// The config file is decoded with yaml.v3, which lowercases field names rather
// than reading the json tags, so every field the documentation shows in
// snake_case needs its own yaml tag. Without them only "preset" survived and
// include_description silently inverted its default.
func TestLoadAppConfReadsEPUBImportStrategyFields(t *testing.T) {
	confPath := filepath.Join(t.TempDir(), "config.yaml")
	body := `
server_conf:
  addr: "127.0.0.1:8080"
app_conf:
  store_path: "store"
  epub_import_strategy:
    preset: markdown
    include_description: true
    keep_images: false
`
	if err := os.WriteFile(confPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	conf, err := loadAppConf(confPath)
	if err != nil {
		t.Fatalf("loadAppConf: %v", err)
	}

	strategy := conf.AppConf.EPUBImportStrategy
	if strategy == nil {
		t.Fatal("epub_import_strategy not decoded")
	}
	if strategy.Preset != epub.PresetMarkdown {
		t.Errorf("preset = %q, want %q", strategy.Preset, epub.PresetMarkdown)
	}
	if !strategy.IncludeDescription {
		t.Error("include_description = false, want true")
	}
	if strategy.KeepImages == nil {
		t.Fatal("keep_images = <nil>, want false")
	}
	if *strategy.KeepImages {
		t.Error("keep_images = true, want false")
	}
}
