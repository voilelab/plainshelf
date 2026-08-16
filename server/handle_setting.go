package server

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// GET /api/setting/cover_to_jpg
func (app *App) HandleGetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.settings.coverToJPG()})
}

// POST /api/setting/cover_to_jpg
//
// The body is the bare literal true or false, not a JSON document, so this one
// does not go through setJSONSetting.
func (app *App) HandleSetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if string(bs) != "true" && string(bs) != "false" {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting(settingKeyCoverToJPG, bs); err != nil {
		app.Error("failed to save setting", "key", settingKeyCoverToJPG, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/cover_to_jpg
func (app *App) HandleDeleteSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	app.settings.deleteSetting(w, settingKeyCoverToJPG)
}

func validateDefaultSplitConfig(cfg shelf.SplitConfig) (string, error) {
	switch cfg.Type {
	case shelf.SplitTypeNone:
		return "", nil
	case shelf.SplitTypeLineCount:
		if cfg.LineCount <= 0 {
			return "line_count must be a positive integer",
				util.Errorf("line_count must be a positive integer, got %d", cfg.LineCount)
		}
		return "", nil
	case shelf.SplitTypeRegex:
		if _, err := regexp.Compile(cfg.Regex); err != nil {
			return fmt.Sprintf("invalid regex: %v", err), util.Errorf("%w", err)
		}
		return "", nil
	default:
		message := fmt.Sprintf("unsupported split type for global default: %q", cfg.Type)
		return message, util.Errorf("%s", message)
	}
}

// GET /api/setting/default_split_config
func (app *App) HandleGetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.settings.defaultSplitConfig()})
}

// POST /api/setting/default_split_config
func (app *App) HandleSetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(app.settings, w, r, settingKeyDefaultSplitConfig, validateDefaultSplitConfig)
}

// DELETE /api/setting/default_split_config
func (app *App) HandleDeleteSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	app.settings.deleteSetting(w, settingKeyDefaultSplitConfig)
}

// validateEPUBImportStrategy names the preset the client sent rather than
// repeating epub's own wording.
func validateEPUBImportStrategy(strategy epub.Strategy) (string, error) {
	if err := strategy.Validate(); err != nil {
		return fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset), util.Errorf("%w", err)
	}
	return "", nil
}

// GET /api/setting/epub_import_strategy
func (app *App) HandleGetSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.settings.epubImportStrategy()})
}

// POST /api/setting/epub_import_strategy
func (app *App) HandleSetSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(app.settings, w, r, settingKeyEPUBImportStrategy, validateEPUBImportStrategy)
}

// DELETE /api/setting/epub_import_strategy
func (app *App) HandleDeleteSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	app.settings.deleteSetting(w, settingKeyEPUBImportStrategy)
}
