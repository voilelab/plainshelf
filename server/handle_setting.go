package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/util"
)

type settingHandlers struct {
	*apiCore

	settings *settings
}

func (h *settingHandlers) getCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.coverToJPG()})
}

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

func (h *settingHandlers) setCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.setBoolSetting(w, r, settingKeyCoverToJPG)
}

func (h *settingHandlers) deleteCoverToJPG(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyCoverToJPG)
}

func (h *settingHandlers) getShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.readShowNSFWFailClosed()})
}

func (h *settingHandlers) setShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.setBoolSetting(w, r, settingKeyShowNSFW)
}

func (h *settingHandlers) deleteShowNSFW(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyShowNSFW)
}

func validateAPIImportStrategy(strategy epub.Strategy) (string, error) {
	if err := strategy.Validate(); err != nil {
		return fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset), util.Errorf("%w", err)
	}
	return "", nil
}

func (h *settingHandlers) getEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.epubImportStrategy()})
}

func (h *settingHandlers) setEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(h.settings, w, r, settingKeyEPUBImportStrategy, validateAPIImportStrategy)
}

func (h *settingHandlers) deleteEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	h.settings.deleteSetting(w, settingKeyEPUBImportStrategy)
}

func (h *settingHandlers) getLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"value": h.settings.logRetentionDays()})
}

func (h *settingHandlers) setLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	if setJSONSetting(h.settings, w, r, settingKeyLogRetentionDays, validateLogRetentionDays) {
		h.settings.applyLogRetention()
	}
}

func (h *settingHandlers) deleteLogRetentionDays(w http.ResponseWriter, r *http.Request) {
	if h.settings.deleteSetting(w, settingKeyLogRetentionDays) {
		h.settings.applyLogRetention()
	}
}
