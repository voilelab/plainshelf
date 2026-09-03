package server

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
)

const (
	settingKeyCoverToJPG         = "cover_to_jpg"
	settingKeyEPUBImportStrategy = "epub_import_strategy"
	settingKeyLogRetentionDays   = "log_retention_days"
)

// maxLogRetentionDays bounds the stored retention window. Ten years is well
// past any use for a daily log file and keeps the value something the date
// arithmetic in logutil can carry.
const maxLogRetentionDays = 3650

// settings resolves a setting to the value the rest of the server should act
// on: the stored one when there is a usable one, the configured one otherwise.
//
// It is a component rather than a set of App methods because the import path
// reads settings too, and the store and the config are the only things any of
// this needs.
type settings struct {
	*logutil.Logger

	db   *store.DB
	conf *AppConf
}

// readJSONSetting reports a stored value as absent when it no longer parses or
// validates, so one bad row cannot wedge a setting.
func readJSONSetting[T any](s *settings, key string, validate func(T) error) (T, bool) {
	var value T

	bs, exists, err := s.db.GetSetting(key)
	if err != nil {
		s.Error("failed to read setting", "key", key, "err", err)
		return value, false
	}
	if !exists {
		return value, false
	}

	if err := json.Unmarshal(bs, &value); err != nil {
		s.Error("stored setting is not valid JSON", "key", key, "err", err)
		return value, false
	}
	if validate != nil {
		if err := validate(value); err != nil {
			s.Error("stored setting is no longer valid", "key", key, "err", err)
			return value, false
		}
	}

	return value, true
}

// A settingValidator returns the message shown to the client separately from
// the error, which is logged and carries util.Errorf's function prefix.
type settingValidator[T any] func(T) (message string, err error)

// setJSONSetting stores the decoded value rather than the raw body, so what is
// persisted is always exactly the fields this build understands. It reports
// whether the value was stored, for a setting that also has to be applied to
// something already running.
func setJSONSetting[T any](s *settings, w http.ResponseWriter, r *http.Request, key string, validate settingValidator[T]) bool {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		s.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	defer r.Body.Close()

	var value T
	if err := decodeRequestJSON(bytes.NewReader(bs), &value, false); err != nil {
		http.Error(w, jsonDecodeMessage(err), http.StatusBadRequest)
		return false
	}

	if message, err := validate(value); err != nil {
		s.Warn("rejected setting value", "key", key, "err", err)
		http.Error(w, message, http.StatusBadRequest)
		return false
	}

	jsonBytes, err := json.Marshal(value, jsonopt.DiskCompact())
	if err != nil {
		s.Error("failed to serialize setting", "key", key, "err", err)
		http.Error(w, "failed to serialize setting", http.StatusInternalServerError)
		return false
	}

	if err := s.db.SetSetting(key, jsonBytes); err != nil {
		s.Error("failed to save setting", "key", key, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return false
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}

// validateLogRetentionDays rejects a window the writers could not act on. Zero
// is meaningful: it is how an operator turns deletion off.
func validateLogRetentionDays(days int) (string, error) {
	if err := checkLogRetentionDays(days); err != nil {
		return fmt.Sprintf("retention days must be between 0 and %d", maxLogRetentionDays), err
	}
	return "", nil
}

func checkLogRetentionDays(days int) error {
	if days < 0 || days > maxLogRetentionDays {
		return util.Errorf("retention days out of range: %d", days)
	}
	return nil
}

// logRetentionDays is the window the log writers are currently applying: the
// stored setting when there is a usable one, otherwise what the configuration
// names for the app logger.
//
// A per-shelf logger may be configured with a different window, which is what
// it returns to when the setting is cleared. Reporting the app logger's is the
// honest single answer for a single control.
func (s *settings) logRetentionDays() int {
	if days, ok := readJSONSetting(s, settingKeyLogRetentionDays, checkLogRetentionDays); ok {
		return days
	}
	return s.conf.Logger.LogFile.ResolvedRetentionDays()
}

// applyLogRetention pushes the stored window to every log writer, which reads
// it on its next rotation. With no stored value each writer returns to the one
// its own configuration names, which is why this clears rather than pushing the
// app logger's fallback over the others.
func (s *settings) applyLogRetention() {
	retention := s.conf.Logger.LogFile.Retention
	if days, ok := readJSONSetting(s, settingKeyLogRetentionDays, checkLogRetentionDays); ok {
		retention.Set(days)
		return
	}
	retention.Clear()
}

// shareLogRetention gives every logger an app builds the same runtime retention
// window, so one setting change reaches all of them. It is attached to the
// configuration because the loggers are built from it, some of them before the
// store this setting lives in has even been opened.
func shareLogRetention(conf *AppConf, retention *logutil.Retention) {
	conf.Logger.LogFile.Retention = retention
	if conf.Worker != nil {
		conf.Worker.Logger.LogFile.Retention = retention
	}
	for _, shelfEntry := range conf.Shelves {
		if shelfEntry != nil {
			shelfEntry.Logger.LogFile.Retention = retention
		}
	}
}

// setRaw stores a value that is not a JSON document. cover_to_jpg is the only
// one: its body is the bare literal true or false.
func (s *settings) setRaw(key string, value []byte) error {
	return s.db.SetSetting(key, value)
}

// deleteSetting reports whether the stored value is gone, for a setting that
// also has to be applied to something already running.
func (s *settings) deleteSetting(w http.ResponseWriter, key string) bool {
	if err := s.db.DeleteSetting(key); err != nil {
		s.Error("failed to delete setting", "key", key, "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return false
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}

func (s *settings) coverToJPG() bool {
	val := s.conf.CoverToJPG

	bs, exists, err := s.db.GetSetting(settingKeyCoverToJPG)
	if err != nil {
		s.Error("failed to read setting", "key", settingKeyCoverToJPG, "err", err)
	} else if exists {
		val = string(bs) == "true"
	}

	return val
}

// epubImportStrategy is the conversion strategy an import uses when the request
// does not carry one of its own.
//
// It is applied server-side during import, the same shape as cover_to_jpg. That
// matters for the desktop client, which imports without opening the import
// dialog and so has no other way to choose.
func (s *settings) epubImportStrategy() epub.Strategy {
	if strategy, ok := readJSONSetting(s, settingKeyEPUBImportStrategy, epub.Strategy.Validate); ok {
		return strategy
	}

	if s.conf.EPUBImportStrategy != nil {
		if err := s.conf.EPUBImportStrategy.Validate(); err == nil {
			return *s.conf.EPUBImportStrategy
		}
		s.Error("epubImportStrategy: invalid configured strategy", "preset", s.conf.EPUBImportStrategy.Preset)
	}

	return epub.DefaultStrategy()
}
