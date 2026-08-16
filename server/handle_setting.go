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

// settingHandlers is the HTTP face of the settings service.
type settingHandlers struct {
	*apiCore

	settings *settings
}

// GET /api/setting/cover_to_jpg
func (h *settingHandlers) getCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.coverToJPG()})
}

// POST /api/setting/cover_to_jpg
//
// The body is the bare literal true or false, not a JSON document, so this one
// does not go through setJSONSetting.
func (h *settingHandlers) setCoverToJPG(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		h.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if string(bs) != "true" && string(bs) != "false" {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := h.settings.setRaw(settingKeyCoverToJPG, bs); err != nil {
		h.Error("failed to save setting", "key", settingKeyCoverToJPG, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/cover_to_jpg
func (h *settingHandlers) deleteCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyCoverToJPG)
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
func (h *settingHandlers) getDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.defaultSplitConfig()})
}

// POST /api/setting/default_split_config
func (h *settingHandlers) setDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(h.settings, w, r, settingKeyDefaultSplitConfig, validateDefaultSplitConfig)
}

// DELETE /api/setting/default_split_config
func (h *settingHandlers) deleteDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyDefaultSplitConfig)
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
func (h *settingHandlers) getEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.epubImportStrategy()})
}

// POST /api/setting/epub_import_strategy
func (h *settingHandlers) setEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(h.settings, w, r, settingKeyEPUBImportStrategy, validateEPUBImportStrategy)
}

// DELETE /api/setting/epub_import_strategy
func (h *settingHandlers) deleteEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyEPUBImportStrategy)
}
