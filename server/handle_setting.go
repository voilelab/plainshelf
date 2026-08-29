package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/util"
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

// GET /api/setting/log_retention_days
//
// The window applies to every log file the server writes. It is a setting
// rather than configuration alone because the desktop build has no config file
// to edit, and deleting log files is not something a user should be unable to
// turn off.
func (h *settingHandlers) getLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.logRetentionDays()})
}

// POST /api/setting/log_retention_days
//
// The new window is pushed to the running writers, which read it on their next
// rotation: a user who turns deletion off must not have to restart the server
// before that takes effect.
func (h *settingHandlers) setLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	if setJSONSetting(h.settings, w, r, settingKeyLogRetentionDays, validateLogRetentionDays) {
		h.settings.applyLogRetention()
	}
}

// DELETE /api/setting/log_retention_days
func (h *settingHandlers) deleteLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	if h.settings.deleteSetting(w, settingKeyLogRetentionDays) {
		h.settings.applyLogRetention()
	}
}
