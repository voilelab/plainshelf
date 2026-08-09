package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// Each setting is read, written, and deleted by three separate handlers, so
// the key is declared once rather than spelled out at each of them.
const (
	settingKeyCoverToJPG         = "cover_to_jpg"
	settingKeyDefaultSplitConfig = "default_split_config"
	settingKeyEPUBImportStrategy = "epub_import_strategy"
)

// readJSONSetting returns the stored value for key when there is one this build
// can still use.
//
// A stored value that no longer parses, or that fails validate, is logged and
// reported as absent so the caller falls back to the config file or the
// built-in default. That keeps one bad row from wedging a setting.
func readJSONSetting[T any](app *App, key string, validate func(T) error) (T, bool) {
	var value T

	bs, exists, err := app.storeDB.GetSetting(key)
	if err != nil {
		app.Error("failed to read setting", "key", key, "err", err)
		return value, false
	}
	if !exists {
		return value, false
	}

	if err := json.Unmarshal(bs, &value); err != nil {
		app.Error("stored setting is not valid JSON", "key", key, "err", err)
		return value, false
	}
	if validate != nil {
		if err := validate(value); err != nil {
			app.Error("stored setting is no longer valid", "key", key, "err", err)
			return value, false
		}
	}

	return value, true
}

// A settingValidator reports whether a decoded setting value is acceptable.
//
// It returns two things because they have different audiences: the message is
// written to the client, so it must carry no internal detail, while the error
// is only logged and is built with util.Errorf, which prefixes it with the
// function that produced it. Returning one value and showing it to both would
// mean either leaking that prefix or giving up the context.
type settingValidator[T any] func(T) (message string, err error)

// setJSONSetting decodes a JSON setting body, validates it, and stores it.
//
// The decoded value is stored rather than the raw body, so what is persisted
// is always exactly the fields this build understands.
func setJSONSetting[T any](app *App, w http.ResponseWriter, r *http.Request, key string, validate settingValidator[T]) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var value T
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&value); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if message, err := validate(value); err != nil {
		app.Warn("rejected setting value", "key", key, "err", err)
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		app.Error("failed to serialize setting", "key", key, "err", err)
		http.Error(w, "failed to serialize setting", http.StatusInternalServerError)
		return
	}

	if err := app.storeDB.SetSetting(key, jsonBytes); err != nil {
		app.Error("failed to save setting", "key", key, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteSetting drops a stored setting, which returns the API to whatever the
// config file or the built-in default supplies.
func (app *App) deleteSetting(w http.ResponseWriter, key string) {
	if err := app.storeDB.DeleteSetting(key); err != nil {
		app.Error("failed to delete setting", "key", key, "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *App) coverToJPG() bool {
	val := app.conf.CoverToJPG

	bs, exists, err := app.storeDB.GetSetting(settingKeyCoverToJPG)
	if err != nil {
		app.Error("failed to read setting", "key", settingKeyCoverToJPG, "err", err)
	} else if exists {
		val = string(bs) == "true"
	}

	return val
}

// GET /api/setting/cover_to_jpg
func (app *App) HandleGetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.coverToJPG()})
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
	app.deleteSetting(w, settingKeyCoverToJPG)
}

// validateDefaultSplitConfig rejects the split types that make no sense as a
// global default, and the malformed parameters of the ones that do.
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

func (app *App) defaultSplitConfig() shelf.SplitConfig {
	if cfg, ok := readJSONSetting[shelf.SplitConfig](app, settingKeyDefaultSplitConfig, nil); ok {
		return cfg
	}

	if app.conf.DefaultSplitConfig != nil {
		return *app.conf.DefaultSplitConfig
	}

	return shelf.SplitConfig{}
}

// GET /api/setting/default_split_config
func (app *App) HandleGetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.defaultSplitConfig()})
}

// POST /api/setting/default_split_config
func (app *App) HandleSetSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(app, w, r, settingKeyDefaultSplitConfig, validateDefaultSplitConfig)
}

// DELETE /api/setting/default_split_config
func (app *App) HandleDeleteSettingDefaultSplitConfig(w http.ResponseWriter, r *http.Request) {
	app.deleteSetting(w, settingKeyDefaultSplitConfig)
}

// validateEPUBImportStrategy tells the client which preset it named rather than
// what epub said about it, which is what the route answered before and keeps
// internal wording out of the response. The underlying error is still logged.
func validateEPUBImportStrategy(strategy epub.Strategy) (string, error) {
	if err := strategy.Validate(); err != nil {
		return fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset), util.Errorf("%w", err)
	}
	return "", nil
}

// epubImportStrategy is the conversion strategy an import uses when the request
// does not carry one of its own.
//
// Unlike default_split_config, which the reader applies client-side, this is
// applied server-side during import - the same shape as cover_to_jpg. That
// matters for the desktop client, which imports without opening the import
// dialog and so has no other way to choose.
func (app *App) epubImportStrategy() epub.Strategy {
	if strategy, ok := readJSONSetting(app, settingKeyEPUBImportStrategy, epub.Strategy.Validate); ok {
		return strategy
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
	app.writeJSON(w, http.StatusOK, map[string]any{"value": app.epubImportStrategy()})
}

// POST /api/setting/epub_import_strategy
func (app *App) HandleSetSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	setJSONSetting(app, w, r, settingKeyEPUBImportStrategy, validateEPUBImportStrategy)
}

// DELETE /api/setting/epub_import_strategy
func (app *App) HandleDeleteSettingEPUBImportStrategy(w http.ResponseWriter, r *http.Request) {
	app.deleteSetting(w, settingKeyEPUBImportStrategy)
}
