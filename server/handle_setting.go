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

// setBoolSetting stores a body that is the bare literal true or false rather
// than a JSON document, so these do not go through setJSONSetting. It is shared
// rather than copied because the two settings shaped this way have to accept and
// reject exactly the same bodies.
func (h *settingHandlers) setBoolSetting(w http.ResponseWriter, r *http.Request, key string) {
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

	if err := h.settings.setRaw(key, bs); err != nil {
		h.Error("failed to save setting", "key", key, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/setting/cover_to_jpg
func (h *settingHandlers) setCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.setBoolSetting(w, r, settingKeyCoverToJPG)
}

// DELETE /api/setting/cover_to_jpg
func (h *settingHandlers) deleteCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyCoverToJPG)
}

// GET /api/setting/show_nsfw
//
// Whether this server serves the books its shelves mark as adult content. The
// mark travels with the shelf; this is the one machine's answer to it, which is
// why it is a setting here rather than a field in shelf.json.
func (h *settingHandlers) getShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.showNSFW()})
}

// POST /api/setting/show_nsfw
func (h *settingHandlers) setShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.setBoolSetting(w, r, settingKeyShowNSFW)
}

// DELETE /api/setting/show_nsfw
//
// Deleting reverts to hidden, the value with nothing stored.
func (h *settingHandlers) deleteShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyShowNSFW)
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
