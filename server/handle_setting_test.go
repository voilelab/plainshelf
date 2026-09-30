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

// A configuration block or a stored value that leaves fields out fills them in
// per field. Omitting the preset used to invalidate the whole strategy, so
// keys written beside it were silently dropped.
func TestPartialStrategyKeepsItsOtherFields(t *testing.T) {
	keepImages := false

	t.Run("stored value", func(t *testing.T) {
		app := newTestApp(t)
		if err := app.storeDB.SetSetting(settingKeyEPUBImportStrategy, []byte(`{"include_description":false}`)); err != nil {
			t.Fatalf("seed strategy: %v", err)
		}

		got := app.handlers.settings.epubImportStrategy()
		if got.Preset != epub.PresetMarkdown {
			t.Errorf("preset = %q, want the %q default", got.Preset, epub.PresetMarkdown)
		}
		if got.IncludeDescription {
			t.Error("include_description = true, want the stored false")
		}
		if got.KeepImages == nil || !*got.KeepImages {
			t.Errorf("keep_images = %v, want the default true", got.KeepImages)
		}
	})

	t.Run("configured block", func(t *testing.T) {
		app := newTestApp(t)
		app.handlers.settings.conf.EPUBImportStrategy = &epub.Strategy{KeepImages: &keepImages}

		got := app.handlers.settings.epubImportStrategy()
		if got.Preset != epub.PresetMarkdown {
			t.Errorf("preset = %q, want the %q default", got.Preset, epub.PresetMarkdown)
		}
		if got.KeepImages == nil || *got.KeepImages {
			t.Errorf("keep_images = %v, want the configured false", got.KeepImages)
		}
	})
}

// Filling in an omitted preset must not make an unknown one acceptable: the
// whole block is still rejected and the built-in default applies.
func TestUnknownConfiguredPresetIsStillRejected(t *testing.T) {
	app := newTestApp(t)
	keepImages := false
	app.handlers.settings.conf.EPUBImportStrategy = &epub.Strategy{
		Preset:     "nonsense",
		KeepImages: &keepImages,
	}

	got := app.handlers.settings.epubImportStrategy()
	if got.Preset != epub.DefaultStrategy().Preset {
		t.Errorf("preset = %q, want the built-in default", got.Preset)
	}
	if !got.IncludeDescription {
		t.Error("include_description = false, want the built-in default true")
	}
	if got.KeepImages == nil || !*got.KeepImages {
		t.Errorf("keep_images = %v, want the built-in default true", got.KeepImages)
	}
}

// A fully written block is unchanged by the normalization above.
func TestCompleteConfiguredStrategyIsUsedAsWritten(t *testing.T) {
	app := newTestApp(t)
	keepImages := false
	app.handlers.settings.conf.EPUBImportStrategy = &epub.Strategy{
		Preset:             epub.PresetPlain,
		IncludeDescription: true,
		KeepImages:         &keepImages,
	}

	got := app.handlers.settings.epubImportStrategy()
	if got.Preset != epub.PresetPlain || !got.IncludeDescription {
		t.Errorf("strategy = %+v, want the configured plain preset with the description", got)
	}
	if got.KeepImages == nil || *got.KeepImages {
		t.Errorf("keep_images = %v, want the configured false", got.KeepImages)
	}
}
