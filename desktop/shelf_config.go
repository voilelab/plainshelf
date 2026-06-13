package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const desktopShelfConfigFilename = "shelves.json"

type desktopShelfConfigFile struct {
	Shelves []*shelf.ShelfConfWithID `json:"shelves"`
}

var nonSlugRunes = regexp.MustCompile(`[^a-z0-9]+`)

func defaultDesktopShelfConf(dataRoot string) *shelf.ShelfConfWithID {
	return &shelf.ShelfConfWithID{
		ID:   "default_shelf",
		Name: "Default Shelf",
		ShelfConf: shelf.ShelfConf{
			Logger: logutil.LogConf{
				Level:  "info",
				Format: "json",
				LogFile: logutil.LogFileConf{
					Type:   logutil.LogFileTypeNameRotate,
					Dir:    filepath.Join(dataRoot, "logs"),
					Prefix: "shelf",
				},
			},
			LibRoot: filepath.Join(dataRoot, "shelf"),
		},
	}
}

func loadOrCreateDesktopShelfConfig(configPath string, fallback *shelf.ShelfConfWithID) ([]*shelf.ShelfConfWithID, error) {
	shelves, err := loadDesktopShelfConfig(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, util.Errorf("%w", err)
		}
	}

	if len(shelves) > 0 {
		return shelves, nil
	}

	fallbackShelves := []*shelf.ShelfConfWithID{cloneShelfConf(fallback)}
	if err := saveDesktopShelfConfig(configPath, fallbackShelves); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fallbackShelves, nil
}

func loadDesktopShelfConfig(configPath string) ([]*shelf.ShelfConfWithID, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config desktopShelfConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return cloneShelfConfs(config.Shelves), nil
}

func saveDesktopShelfConfig(configPath string, shelves []*shelf.ShelfConfWithID) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return util.Errorf("%w", err)
	}

	data, err := json.MarshalIndent(desktopShelfConfigFile{Shelves: shelves}, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}
	data = append(data, '\n')

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return util.Errorf("%w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	return nil
}

func cloneShelfConfs(shelves []*shelf.ShelfConfWithID) []*shelf.ShelfConfWithID {
	clones := make([]*shelf.ShelfConfWithID, 0, len(shelves))
	for _, conf := range shelves {
		if conf == nil {
			continue
		}
		clones = append(clones, cloneShelfConf(conf))
	}
	return clones
}

func cloneShelfConf(conf *shelf.ShelfConfWithID) *shelf.ShelfConfWithID {
	if conf == nil {
		return nil
	}
	clone := *conf
	return &clone
}

func (a *DesktopApp) newDesktopShelfConf(name string, libRoot string) (*shelf.ShelfConfWithID, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, util.NewError("shelf name cannot be empty")
	}

	normalizedLibRoot, err := normalizeDesktopShelfPath(libRoot)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	for _, existing := range a.desktopShelves {
		if existing == nil {
			continue
		}
		existingRoot, err := normalizeDesktopShelfPath(existing.LibRoot)
		if err == nil && existingRoot == normalizedLibRoot {
			return nil, util.Errorf("shelf directory is already configured: %s", normalizedLibRoot)
		}
	}

	id := uniqueDesktopShelfID(trimmedName, normalizedLibRoot, a.desktopShelves)
	return &shelf.ShelfConfWithID{
		ID:   id,
		Name: trimmedName,
		ShelfConf: shelf.ShelfConf{
			Logger: logutil.LogConf{
				Level:  "info",
				Format: "json",
				LogFile: logutil.LogFileConf{
					Type:   logutil.LogFileTypeNameRotate,
					Dir:    filepath.Join(a.dataRoot, "logs"),
					Prefix: "shelf-" + id,
				},
			},
			LibRoot: normalizedLibRoot,
		},
	}, nil
}

func normalizeDesktopShelfPath(libRoot string) (string, error) {
	trimmed := strings.TrimSpace(libRoot)
	if trimmed == "" {
		return "", util.NewError("shelf directory cannot be empty")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	cleanPath := filepath.Clean(absPath)

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if !info.IsDir() {
		return "", util.NewError("shelf directory must be a directory")
	}

	dir, err := os.Open(cleanPath)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	defer dir.Close()
	if _, err := dir.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
		return "", util.Errorf("%w", err)
	}

	return cleanPath, nil
}

func uniqueDesktopShelfID(name string, libRoot string, shelves []*shelf.ShelfConfWithID) string {
	base := slugifyShelfID(name)
	if base == "" {
		base = slugifyShelfID(filepath.Base(libRoot))
	}
	if base == "" {
		base = "shelf"
	}

	used := make(map[string]struct{}, len(shelves))
	for _, existing := range shelves {
		if existing == nil {
			continue
		}
		used[existing.ID] = struct{}{}
	}

	if _, exists := used[base]; !exists {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func slugifyShelfID(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range lower {
		if r <= unicode.MaxASCII {
			builder.WriteRune(r)
		}
	}
	slug := nonSlugRunes.ReplaceAllString(builder.String(), "-")
	slug = strings.Trim(slug, "-")
	return slug
}
