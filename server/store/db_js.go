//go:build js && wasm

package store

import (
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

// DB keeps settings in one JSON file in the browser build: badger mmaps its
// files, which js/wasm cannot do, and a plain file persists through OPFS.
type DB struct {
	mu       sync.Mutex
	path     string
	settings map[string][]byte
}

const settingsFile = "settings.json"

func New(dbPath string) (*DB, error) {
	db := &DB{path: filepath.Join(dbPath, settingsFile), settings: map[string][]byte{}}
	bs, err := os.ReadFile(db.path)
	if errors.Is(err, fs.ErrNotExist) {
		return db, nil
	}
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := json.Unmarshal(bs, &db.settings, jsonopt.Disk()); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return db, nil
}

func (db *DB) Close() error { return nil }

// GetSetting returns the value, whether the key exists, and any error.
func (db *DB) GetSetting(key string) ([]byte, bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	value, ok := db.settings[key]
	return append([]byte(nil), value...), ok, nil
}

func (db *DB) SetSetting(key string, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	prev, had := db.settings[key]
	db.settings[key] = append([]byte(nil), value...)
	if err := db.save(); err != nil {
		if had {
			db.settings[key] = prev
		} else {
			delete(db.settings, key)
		}
		return err
	}
	return nil
}

func (db *DB) DeleteSetting(key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	prev, had := db.settings[key]
	if !had {
		return nil
	}
	delete(db.settings, key)
	if err := db.save(); err != nil {
		db.settings[key] = prev
		return err
	}
	return nil
}

func (db *DB) save() error {
	bs, err := json.Marshal(db.settings, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteTextFileAtomic(db.path, "settings-*.json", string(bs)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
