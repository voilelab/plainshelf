package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/voilelab/plainshelf/internal/epub"
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
		app.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if string(bs) != "true" && string(bs) != "false" {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting("cover_to_jpg", bs); err != nil {
		app.Error("SetSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/cover_to_jpg
func (app *App) HandleDeleteSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("cover_to_jpg"); err != nil {
		app.Error("DeleteSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *App) defaultSplitConfig() shelf.SplitConfig {
	bs, exists, err := app.storeDB.GetSetting("default_split_config")
	if err != nil {
		app.Error("defaultSplitConfig:", "err", err)
	} else if exists {
		var cfg shelf.SplitConfig
		if err := json.Unmarshal(bs, &cfg); err != nil {
			app.Error("defaultSplitConfig: invalid stored JSON", "err", err)
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
		app.Error("read request body:", "err", err)
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
		app.Error("marshal default_split_config:", "err", err)
		http.Error(w, "failed to serialize config", http.StatusInternalServerError)
		return
	}

	if err := app.storeDB.SetSetting("default_split_config", jsonBytes); err != nil {
		app.Error("SetSettingDefaultSplitConfig:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/default_split_config
func (app *App) HandleDeleteSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("default_split_config"); err != nil {
		app.Error("DeleteSettingDefaultSplitConfig:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// epubImportStrategy is the conversion strategy an import uses when the request
// does not carry one of its own.
//
// Unlike default_split_config, which the reader applies client-side, this is
// applied server-side during import - the same shape as cover_to_jpg. That
// matters for the desktop client, which imports without opening the import
// dialog and so has no other way to choose.
func (app *App) epubImportStrategy() epub.Strategy {
	bs, exists, err := app.storeDB.GetSetting("epub_import_strategy")
	if err != nil {
		app.Error("epubImportStrategy:", "err", err)
	} else if exists {
		var strategy epub.Strategy
		if err := json.Unmarshal(bs, &strategy); err != nil {
			app.Error("epubImportStrategy: invalid stored JSON", "err", err)
		} else if err := strategy.Validate(); err != nil {
			app.Error("epubImportStrategy: invalid stored strategy", "err", err)
		} else {
			return strategy
		}
	}

	if app.conf.EPUBImportStrategy != nil {
		if err := app.conf.EPUBImportStrategy.Validate(); err == nil {
			return *app.conf.EPUBImportStrategy
		}
		app.Error("epubImportStrategy: invalid configured strategy", "preset", app.conf.EPUBImportStrategy.Preset)
	}

	return epub.DefaultStrategy()
}

// GET /api/setting/epub_import_strategy
func (app *App) HandleGetSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	strategy := app.epubImportStrategy()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"value": strategy})
}

// POST /api/setting/epub_import_strategy
func (app *App) HandleSetSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var strategy epub.Strategy
	dec := json.NewDecoder(strings.NewReader(string(bs)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&strategy); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := strategy.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset), http.StatusBadRequest)
		return
	}

	// Store the decoded value rather than the raw body so the persisted setting
	// is always exactly the fields this build understands.
	jsonBytes, err := json.Marshal(strategy)
	if err != nil {
		app.Error("marshal epub_import_strategy:", "err", err)
		http.Error(w, "failed to serialize strategy", http.StatusInternalServerError)
		return
	}

	if err := app.storeDB.SetSetting("epub_import_strategy", jsonBytes); err != nil {
		app.Error("SetSettingEPUBImportStrategy:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/epub_import_strategy
func (app *App) HandleDeleteSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("epub_import_strategy"); err != nil {
		app.Error("DeleteSettingEPUBImportStrategy:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
