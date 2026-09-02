package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type desktopShelfEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LibRoot      string `json:"lib_root"`
	ScanInterval string `json:"scan_interval,omitempty"`

	// ReadOnly opens the shelf without writing to it; see shelf.ShelfConf.
	//
	// This file is not inside any shelf, so a shelf being read-only never makes
	// its own entry here unwritable: a shelf that was opened read-only can
	// always be edited back. See DesktopApp.ModifyShelf.
	ReadOnly bool `json:"read_only,omitempty"`
}

const (
	desktopLegacyDefaultShelfID   = "default_shelf"
	desktopLegacyDefaultShelfName = "Default Shelf"
	desktopLegacyShelfDirName     = "shelf"

	// Folder under the desktop data root holding the shelves PlainShelf creates
	// for itself, as opposed to a folder the user already had and points at.
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
	data, err := json.MarshalIndent(conf, "", "  ")
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
			LibRoot:      entry.LibRoot,
			ScanInterval: entry.ScanInterval,
			ReadOnly:     entry.ReadOnly,
		},
	}
}

// defaultDesktopShelfPath is where a shelf with this id is created when the
// user is not bringing an existing folder: beside shelves.json in the desktop
// data root, under a per-shelf directory named after the id. Empty when there
// is nothing to derive it from — no id, or a config path the app has not
// resolved yet — so a caller never offers a relative path as a default.
func defaultDesktopShelfPath(configPath, id string) string {
	if configPath == "" || id == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), desktopShelvesDirName, id)
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
