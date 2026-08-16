package server

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/shelf"
)

// Every getter falls back to a usable default, so a getter that ignored the
// store entirely would still answer plausibly. These assert with values that
// differ from the fallback.

func TestStoredJSONSettingsAreReturned(t *testing.T) {
	env := newAPITestEnv(t)

	if got := env.app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
		t.Fatalf("default preset = %q, want %q", got, epub.PresetMarkdown)
	}
	if err := env.app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"plain"}`)); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	if got := env.app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetPlain {
		t.Fatalf("preset = %q, want %q from the store", got, epub.PresetPlain)
	}

	if err := env.app.storeDB.SetSetting(settingKeyDefaultSplitConfig, []byte(`{"type":"line_count","line_count":42}`)); err != nil {
		t.Fatalf("seed split config: %v", err)
	}
	cfg := env.app.handlers.settings.defaultSplitConfig()
	if cfg.Type != shelf.SplitTypeLineCount || cfg.LineCount != 42 {
		t.Fatalf("split config = %+v, want line_count 42 from the store", cfg)
	}
}

// One unreadable row must not wedge a setting: it is ignored in favour of the
// fallback rather than surfaced.
func TestUnusableStoredSettingsFallBack(t *testing.T) {
	t.Run("corrupt JSON", func(t *testing.T) {
		env := newAPITestEnv(t)
		if err := env.app.storeDB.SetSetting(settingKeyDefaultSplitConfig, []byte("{{{")); err != nil {
			t.Fatalf("seed split config: %v", err)
		}

		// SplitConfig holds a slice, so it is not comparable as a whole.
		if cfg := env.app.handlers.settings.defaultSplitConfig(); cfg.Type != "" || cfg.LineCount != 0 {
			t.Fatalf("split config = %+v, want the zero fallback", cfg)
		}
	})

	t.Run("parses but fails validation", func(t *testing.T) {
		env := newAPITestEnv(t)
		if err := env.app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"no-such-preset"}`)); err != nil {
			t.Fatalf("seed strategy: %v", err)
		}

		if got := env.app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
			t.Fatalf("preset = %q, want the %q fallback", got, epub.PresetMarkdown)
		}
	})
}

func TestDeleteSettingRestoresTheFallback(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"plain"}`)); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	if got := env.app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetPlain {
		t.Fatalf("preset = %q, want %q before delete", got, epub.PresetPlain)
	}

	if err := env.app.storeDB.DeleteSetting(settingKeyEPUBImportStrategy); err != nil {
		t.Fatalf("delete strategy: %v", err)
	}

	if got := env.app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
		t.Fatalf("preset = %q, want the %q fallback after delete", got, epub.PresetMarkdown)
	}
}
