package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
)

func (app *App) coverToJPG() bool {
	val := app.conf.CoverToJPG

	bs, exists, err := app.storeDB.GetSetting("cover_to_jpg")
	if err != nil {
		app.Error("coverToJPG:", "err", err)
	} else if exists {
		val = string(bs) == "true"
	}

	return val
}

func (app *App) readHistoryLimit() int {
	val := app.conf.ReadHistoryLimit

	bs, exists, err := app.storeDB.GetSetting("read_history_limit")
	if err != nil {
		app.Error("readHistoryLimit:", "err", err)
	} else if exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(string(bs)))
		if err != nil || parsed < 0 {
			app.Error("readHistoryLimit: invalid stored value", "value", string(bs), "err", err)
		} else {
			val = parsed
		}
	}

	if val < 0 {
		return 0
	}
	return val
}

// GET /api/setting/cover_to_jpg
func (app *App) HandleGetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	val := app.coverToJPG()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if val {
		w.Write([]byte(`{"value": true}`))
	} else {
		w.Write([]byte(`{"value": false}`))
	}
}

// POST /api/setting/cover_to_jpg
func (app *App) HandleSetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Logger.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if string(bs) != "true" && string(bs) != "false" {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting("cover_to_jpg", bs); err != nil {
		app.Logger.Error("SetSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/cover_to_jpg
func (app *App) HandleDeleteSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("cover_to_jpg"); err != nil {
		app.Logger.Error("DeleteSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/setting/read_history_limit
func (app *App) HandleGetSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	val := app.readHistoryLimit()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(fmt.Appendf(nil, `{"value": %d}`, val))
}

// POST /api/setting/read_history_limit
func (app *App) HandleSetSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Logger.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	val, err := strconv.Atoi(strings.TrimSpace(string(bs)))
	if err != nil || val < 0 {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting("read_history_limit", []byte(strconv.Itoa(val))); err != nil {
		app.Logger.Error("SetSettingReadHistoryLimit:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/read_history_limit
func (app *App) HandleDeleteSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("read_history_limit"); err != nil {
		app.Logger.Error("DeleteSettingReadHistoryLimit:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *App) defaultSplitConfig() shelf.SplitConfig {
	bs, exists, err := app.storeDB.GetSetting("default_split_config")
	if err != nil {
		app.Logger.Error("defaultSplitConfig:", "err", err)
	} else if exists {
		var cfg shelf.SplitConfig
		if err := json.Unmarshal(bs, &cfg); err != nil {
			app.Logger.Error("defaultSplitConfig: invalid stored JSON", "err", err)
		} else {
			return cfg
		}
	}

	if app.conf.DefaultSplitConfig != nil {
		return *app.conf.DefaultSplitConfig
	}

	return shelf.SplitConfig{}
}

// GET /api/setting/default_split_config
func (app *App) HandleGetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	cfg := app.defaultSplitConfig()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"value": cfg})
}

// POST /api/setting/default_split_config
func (app *App) HandleSetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Logger.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var cfg shelf.SplitConfig
	dec := json.NewDecoder(strings.NewReader(string(bs)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	switch cfg.Type {
	case shelf.SplitTypeNone:
	case shelf.SplitTypeLineCount:
		if cfg.LineCount <= 0 {
			http.Error(w, "line_count must be a positive integer", http.StatusBadRequest)
			return
		}
	case shelf.SplitTypeRegex:
		if _, err := regexp.Compile(cfg.Regex); err != nil {
			http.Error(w, fmt.Sprintf("invalid regex: %v", err), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("unsupported split type for global default: %q", cfg.Type), http.StatusBadRequest)
		return
	}

	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		app.Logger.Error("marshal default_split_config:", "err", err)
		http.Error(w, "failed to serialize config", http.StatusInternalServerError)
		return
	}

	if err := app.storeDB.SetSetting("default_split_config", jsonBytes); err != nil {
		app.Logger.Error("SetSettingDefaultSplitConfig:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/default_split_config
func (app *App) HandleDeleteSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("default_split_config"); err != nil {
		app.Logger.Error("DeleteSettingDefaultSplitConfig:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
