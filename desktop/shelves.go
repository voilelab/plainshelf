package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

const desktopShelfConfigFilename = "shelves.json"

type DesktopShelfInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LibRoot string `json:"lib_root"`
}

type desktopShelfConfig struct {
	Shelves []DesktopShelfInfo `json:"shelves"`
}

var nonShelfIDCharPattern = regexp.MustCompile(`[^a-z0-9_-]+`)

func desktopShelfConfigPath(dataRoot string) string {
	return filepath.Join(dataRoot, desktopShelfConfigFilename)
}

func loadDesktopShelves(dataRoot string) ([]*server.ShelfConfWithID, error) {
	configPath := desktopShelfConfigPath(dataRoot)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultDesktopShelves(dataRoot), nil
		}
		return nil, util.Errorf("%w", err)
	}

	var config desktopShelfConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if len(config.Shelves) == 0 {
		return defaultDesktopShelves(dataRoot), nil
	}

	shelves := make([]*server.ShelfConfWithID, 0, len(config.Shelves))
	for _, entry := range config.Shelves {
		conf, err := desktopShelfInfoToConf(entry, dataRoot)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		shelves = append(shelves, conf)
	}
	return shelves, nil
}

func saveDesktopShelves(configPath string, shelves []*server.ShelfConfWithID) error {
	entries := make([]DesktopShelfInfo, 0, len(shelves))
	for _, conf := range shelves {
		if conf == nil {
			continue
		}
		entries = append(entries, DesktopShelfInfo{
			ID:      conf.ID,
			Name:    conf.Name,
			LibRoot: conf.LibRoot,
		})
	}

	data, err := json.MarshalIndent(desktopShelfConfig{Shelves: entries}, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return util.Errorf("%w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func defaultDesktopShelves(dataRoot string) []*server.ShelfConfWithID {
	return []*server.ShelfConfWithID{
		{
			ID:   "default_shelf",
			Name: "Default Shelf",
			ShelfConf: shelf.ShelfConf{
				Logger:  desktopShelfLogConf(dataRoot),
				LibRoot: filepath.Join(dataRoot, "shelf"),
			},
		},
	}
}

func desktopShelfInfoToConf(entry DesktopShelfInfo, dataRoot string) (*server.ShelfConfWithID, error) {
	id := strings.TrimSpace(entry.ID)
	name := strings.TrimSpace(entry.Name)
	libRoot := strings.TrimSpace(entry.LibRoot)
	if id == "" {
		return nil, util.NewError("shelf ID cannot be empty")
	}
	if name == "" {
		name = id
	}
	if libRoot == "" {
		return nil, util.NewError("shelf library root cannot be empty")
	}

	absLibRoot, err := filepath.Abs(libRoot)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &server.ShelfConfWithID{
		ID:   id,
		Name: name,
		ShelfConf: shelf.ShelfConf{
			Logger:  desktopShelfLogConf(dataRoot),
			LibRoot: absLibRoot,
		},
	}, nil
}

func desktopShelfLogConf(dataRoot string) logutil.LogConf {
	return logutil.LogConf{
		Level:  "info",
		Format: "json",
		LogFile: logutil.LogFileConf{
			Type:   logutil.LogFileTypeNameRotate,
			Dir:    filepath.Join(dataRoot, "logs"),
			Prefix: "shelf",
		},
	}
}

func normalizeDesktopShelfDirectory(dir string) (string, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return "", util.NewError("shelf directory cannot be empty")
	}

	absDir, err := filepath.Abs(trimmed)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if !info.IsDir() {
		return "", util.NewError("shelf path is not a directory")
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	_ = entries
	return absDir, nil
}

func generateDesktopShelfID(name string, libRoot string, existing map[string]struct{}) string {
	base := slugifyShelfID(name)
	if base == "" {
		base = slugifyShelfID(filepath.Base(libRoot))
	}
	if base == "" {
		base = "shelf"
	}

	if _, ok := existing[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}

func slugifyShelfID(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = nonShelfIDCharPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-_")
	if slug == "" {
		return ""
	}
	return slug
}
