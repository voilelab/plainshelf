package main

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type desktopShelfEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LibRoot      string `json:"lib_root"`
	ScanInterval string `json:"scan_interval,omitempty"`

	// BookCheckInterval controls how often per-book staleness checks run; see
	// shelf.ShelfConf. Empty means "same as scan_interval" (the default), so it is
	// omitted from shelves.json until the user sets it.
	BookCheckInterval string `json:"book_check_interval,omitempty"`

	// ReadOnly opens the shelf without writing to it; see shelf.ShelfConf.
	//
	// This file is not inside any shelf, so a shelf being read-only never makes
	// its own entry here unwritable: a shelf that was opened read-only can
	// always be edited back. See DesktopApp.ModifyShelf.
	ReadOnly bool `json:"read_only,omitzero"`
}

const (
	desktopLegacyDefaultShelfID   = "default_shelf"
	desktopLegacyDefaultShelfName = "Default Shelf"
	desktopLegacyShelfDirName     = "shelf"

	// desktopShelvesDirName holds the shelves the user creates from the add-shelf
	// form without choosing a directory. It is a level below the data root rather
	// than beside it, so a shelf whose name slugifies to "shelf" cannot land on
	// the legacy default shelf's own directory (desktopLegacyShelfDirName).
	desktopShelvesDirName = "shelves"
)

type desktopShelvesConfig struct {
	Shelves []desktopShelfEntry `json:"shelves"`
}

func loadDesktopShelves(configPath string) (*desktopShelvesConfig, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &desktopShelvesConfig{}, nil
	}
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	var conf desktopShelvesConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &conf, nil
}

func loadOrMigrateDesktopShelves(configPath, dataRoot string) (*desktopShelvesConfig, error) {
	_, err := os.Stat(configPath)
	if err == nil {
		conf, err := loadDesktopShelves(configPath)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		return conf, nil
	}
	if !os.IsNotExist(err) {
		return nil, util.Errorf("%w", err)
	}

	conf := defaultDesktopShelvesConfig(dataRoot)
	if err := saveDesktopShelves(configPath, conf); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return conf, nil
}

func defaultDesktopShelvesConfig(dataRoot string) *desktopShelvesConfig {
	return &desktopShelvesConfig{
		Shelves: []desktopShelfEntry{
			{
				ID:      desktopLegacyDefaultShelfID,
				Name:    desktopLegacyDefaultShelfName,
				LibRoot: filepath.Join(dataRoot, desktopLegacyShelfDirName),
			},
		},
	}
}

func saveDesktopShelves(configPath string, conf *desktopShelvesConfig) error {
	data, err := json.Marshal(conf, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func toShelfConfWithID(entry desktopShelfEntry) shelf.ShelfConfWithID {
	return shelf.ShelfConfWithID{
		ID:   entry.ID,
		Name: entry.Name,
		ShelfConf: shelf.ShelfConf{
			LibRoot:           entry.LibRoot,
			ScanInterval:      entry.ScanInterval,
			BookCheckInterval: entry.BookCheckInterval,
			ReadOnly:          entry.ReadOnly,
		},
	}
}

// defaultDesktopShelfDir is where a shelf with the given id is created when the
// user does not pick a directory. shelves.json sits directly in the desktop data
// root (see startServer), which is what makes that root recoverable from the
// config path alone instead of being threaded through as a second field.
func defaultDesktopShelfDir(shelvesConfigPath, shelfID string) string {
	return filepath.Join(filepath.Dir(shelvesConfigPath), desktopShelvesDirName, shelfID)
}

func normalizeDesktopShelfDirectory(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", util.Errorf("shelf directory cannot be empty")
	}
	if !filepath.IsAbs(dir) {
		return "", util.Errorf("shelf directory must be an absolute path: %q", dir)
	}
	return dir, nil
}

func slugifyShelfID(name string) string {
	var b strings.Builder
	prevHyphen := true
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		return "shelf"
	}
	return result
}

func generateDesktopShelfID(name string, existingIDs map[string]bool) string {
	// The standalone reader stores progress under this id as a synthetic shelf
	// key; a real shelf that took it would share (and break) that namespace, so
	// it is never handed out here.
	reserved := func(id string) bool { return existingIDs[id] || id == readingprogress.ReaderShelfID }

	base := slugifyShelfID(name)
	if !reserved(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !reserved(candidate) {
			return candidate
		}
	}
}
