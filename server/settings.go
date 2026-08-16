package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

const (
	settingKeyCoverToJPG         = "cover_to_jpg"
	settingKeyDefaultSplitConfig = "default_split_config"
	settingKeyEPUBImportStrategy = "epub_import_strategy"
)

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
// persisted is always exactly the fields this build understands.
func setJSONSetting[T any](s *settings, w http.ResponseWriter, r *http.Request, key string, validate settingValidator[T]) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		s.Error("read request body:", "err", err)
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
		s.Warn("rejected setting value", "key", key, "err", err)
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		s.Error("failed to serialize setting", "key", key, "err", err)
		http.Error(w, "failed to serialize setting", http.StatusInternalServerError)
		return
	}

	if err := s.db.SetSetting(key, jsonBytes); err != nil {
		s.Error("failed to save setting", "key", key, "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// setRaw stores a value that is not a JSON document. cover_to_jpg is the only
// one: its body is the bare literal true or false.
func (s *settings) setRaw(key string, value []byte) error {
	return s.db.SetSetting(key, value)
}

func (s *settings) deleteSetting(w http.ResponseWriter, key string) {
	if err := s.db.DeleteSetting(key); err != nil {
		s.Error("failed to delete setting", "key", key, "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

func (s *settings) defaultSplitConfig() shelf.SplitConfig {
	if cfg, ok := readJSONSetting[shelf.SplitConfig](s, settingKeyDefaultSplitConfig, nil); ok {
		return cfg
	}

	if s.conf.DefaultSplitConfig != nil {
		return *s.conf.DefaultSplitConfig
	}

	return shelf.SplitConfig{}
}

// epubImportStrategy is the conversion strategy an import uses when the request
// does not carry one of its own.
//
// Unlike default_split_config, which the reader applies client-side, this is
// applied server-side during import - the same shape as cover_to_jpg. That
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
