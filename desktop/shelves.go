package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type desktopShelfEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LibRoot      string `json:"lib_root"`
	ScanInterval string `json:"scan_interval,omitempty"`
}

const (
	desktopLegacyDefaultShelfID   = "default_shelf"
	desktopLegacyDefaultShelfName = "Default Shelf"
	desktopLegacyShelfDirName     = "shelf"
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
		return loadDesktopShelves(configPath)
	}
	if !os.IsNotExist(err) {
		return nil, util.Errorf("%w", err)
	}

	conf := defaultDesktopShelvesConfig(dataRoot)
	if err := saveDesktopShelves(configPath, conf); err != nil {
		return nil, util.Errorf("migrating legacy shelf config: %w", err)
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
		},
	}
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
	base := slugifyShelfID(name)
	if !existingIDs[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !existingIDs[candidate] {
			return candidate
		}
	}
}
