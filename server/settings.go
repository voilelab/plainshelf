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
	settingKeyShowNSFW           = "show_nsfw"
)

const maxLogRetentionDays = 3650

type settings struct {
	*logutil.Logger

	db   *store.DB
	conf *AppConf
}

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

type settingValidator[T any] func(T) (message string, err error)

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

func (s *settings) logRetentionDays() int {
	if days, ok := readJSONSetting(s, settingKeyLogRetentionDays, checkLogRetentionDays); ok {
		return days
	}
	return s.conf.Logger.LogFile.ResolvedRetentionDays()
}

func (s *settings) applyLogRetention() {
	retention := s.conf.Logger.LogFile.Retention
	if days, ok := readJSONSetting(s, settingKeyLogRetentionDays, checkLogRetentionDays); ok {
		retention.Set(days)
		return
	}
	retention.Clear()
}

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

func (s *settings) setRaw(key string, value []byte) error {
	return s.db.SetSetting(key, value)
}

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

func (s *settings) epubImportStrategy() epub.Strategy {
	if strategy, ok := readJSONSetting(s, settingKeyEPUBImportStrategy, validateStrategyWithDefaults); ok {
		return strategy.Normalized()
	}

	if s.conf.EPUBImportStrategy != nil {
		if strategy := s.conf.EPUBImportStrategy.Normalized(); strategy.Validate() == nil {
			return strategy
		}
		s.Error("epubImportStrategy: invalid configured strategy", "preset", s.conf.EPUBImportStrategy.Preset)
	}

	return epub.DefaultStrategy().Normalized()
}

func validateStrategyWithDefaults(strategy epub.Strategy) error {
	return strategy.Normalized().Validate()
}

func (s *settings) readShowNSFW() bool {
	bs, exists, err := s.db.GetSetting(settingKeyShowNSFW)
	if err != nil {
		s.Error("failed to read setting", "key", settingKeyShowNSFW, "err", err)
		return false
	}
	return exists && string(bs) == "true"
}
