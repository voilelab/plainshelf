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

type DB struct {
	mu       sync.Mutex
	path     string
	settings map[string][]byte
	unlock   func() error
	closed   bool
	badger   bool
}

const (
	settingsFile   = "settings.json"
	badgerManifest = "MANIFEST"
)

func New(dbPath string) (*DB, error) {
	unlock, err := lockStore(dbPath)
	if err != nil {
		return nil, err
	}
	db := &DB{path: filepath.Join(dbPath, settingsFile), settings: map[string][]byte{}}
	db.unlock = unlock
	bs, err := os.ReadFile(db.path)
	if errors.Is(err, fs.ErrNotExist) {
		_, statErr := os.Stat(filepath.Join(dbPath, badgerManifest))
		db.badger = statErr == nil
		return db, nil
	}
	if err != nil {
		_ = unlock()
		return nil, util.Errorf("%w", err)
	}
	if err := json.Unmarshal(bs, &db.settings, jsonopt.Disk()); err != nil {
		_ = unlock()
		return nil, util.Errorf("%w", err)
	}
	if db.settings == nil {
		_ = unlock()
		return nil, util.NewError("settings.json must be an object")
	}
	return db, nil
}

func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil
	}
	if err := db.unlock(); err != nil {
		return util.Errorf("%w", err)
	}
	db.closed = true
	return nil
}

func (db *DB) HasUnimportedBadger() bool { return db.badger }

var errClosed = util.NewError("settings store is closed")

func (db *DB) GetSetting(key string) ([]byte, bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil, false, errClosed
	}
	value, ok := db.settings[key]
	return append([]byte(nil), value...), ok, nil
}

func (db *DB) SetSetting(key string, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return errClosed
	}
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
	if db.closed {
		return errClosed
	}
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
	if err := fsutil.WriteTextFileDurable(db.path, "settings-*.json", string(bs)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
