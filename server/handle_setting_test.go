package server

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/epub"
)

// Every getter falls back to a usable default, so a getter that ignored the
// store entirely would still answer plausibly. These assert with values that
// differ from the fallback.

func TestStoredJSONSettingsAreReturned(t *testing.T) {
	app := newTestApp(t)

	if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
		t.Fatalf("default preset = %q, want %q", got, epub.PresetMarkdown)
	}
	if err := app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"plain"}`)); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetPlain {
		t.Fatalf("preset = %q, want %q from the store", got, epub.PresetPlain)
	}
}

// One unreadable row must not wedge a setting: it is ignored in favour of the
// fallback rather than surfaced.
func TestUnusableStoredSettingsFallBack(t *testing.T) {
	t.Run("corrupt JSON", func(t *testing.T) {
		app := newTestApp(t)
		if err := app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte("{{{")); err != nil {
			t.Fatalf("seed strategy: %v", err)
		}

		if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.DefaultStrategy().Preset {
			t.Fatalf("preset = %q, want the configured fallback", got)
		}
	})

	t.Run("parses but fails validation", func(t *testing.T) {
		app := newTestApp(t)
		if err := app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"no-such-preset"}`)); err != nil {
			t.Fatalf("seed strategy: %v", err)
		}

		if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
			t.Fatalf("preset = %q, want the %q fallback", got, epub.PresetMarkdown)
		}
	})
}

func TestDeleteSettingRestoresTheFallback(t *testing.T) {
	app := newTestApp(t)

	if err := app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"preset":"plain"}`)); err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetPlain {
		t.Fatalf("preset = %q, want %q before delete", got, epub.PresetPlain)
	}

	if err := app.storeDB.DeleteSetting(settingKeyEPUBImportStrategy); err != nil {
		t.Fatalf("delete strategy: %v", err)
	}

	if got := app.handlers.settings.epubImportStrategy().Preset; got != epub.PresetMarkdown {
		t.Fatalf("preset = %q, want the %q fallback after delete", got, epub.PresetMarkdown)
	}
}
