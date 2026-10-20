package shelf

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

const shelfConfigFile = "shelf.json"

const shelfConfigSchemaVersion = 1

const maxShelfConfigBytes = 1 << 20

// shelfConfigJSON mirrors shelf.json. Unknown fields are accepted on purpose, so
// a shelf shared with a newer build is not rejected by an older one.
type shelfConfigJSON struct {
	SchemaVersion int `json:"schema_version"`
	Scan          struct {
		// IgnoredDirs are the directory names under books/ this shelf skips.
		//
		// Present, it replaces the built-in defaults outright — including an
		// empty list, which means "skip nothing but hidden directories". Absent
		// (a nil slice, which is why the entries stay raw) gets
		// DefaultIgnoredDirs. There is no second field layering names on top.
		//
		// An entry is always an object — {"name": "@eaDir"}, optionally with a
		// "reason" — so a file reads the same way wherever the reader looks and
		// a field added later does not turn some entries into another kind of
		// value.
		IgnoredDirs []jsontext.Value `json:"ignored_dirs"`
	} `json:"scan"`
	Content struct {
		// NSFWFolders are the folder subtrees this shelf marks as adult content;
		// a folder marks itself and everything below it.
		//
		// Unlike IgnoredDirs there is no built-in list to replace: absent and
		// empty both mean "this shelf marks no folder", because only the user
		// knows what their own folders hold.
		//
		// An entry is always an object, for the same reason IgnoredDirs entries
		// are: one shape wherever the reader looks.
		NSFWFolders []jsontext.Value `json:"nsfw_folders"`
	} `json:"content"`
}

// ignoredDirJSON is the object form of one entry.
type ignoredDirJSON struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func parseIgnoredDir(raw jsontext.Value) (shelfutil.IgnoredDir, error) {
	var object ignoredDirJSON
	if err := json.Unmarshal(raw, &object); err != nil {
		return shelfutil.IgnoredDir{}, util.Errorf("entry is not a {name, reason} object: %w", err)
	}
	return shelfutil.IgnoredDir{Name: object.Name, Reason: object.Reason}, nil
}

// nsfwFolderJSON is the object form of one content.nsfw_folders entry.
type nsfwFolderJSON struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func parseNSFWFolder(raw jsontext.Value) (shelfutil.NSFWFolder, error) {
	var object nsfwFolderJSON
	if err := json.Unmarshal(raw, &object); err != nil {
		return shelfutil.NSFWFolder{}, util.Errorf("entry is not a {path, reason} object: %w", err)
	}
	return shelfutil.NSFWFolder{Path: object.Path, Reason: object.Reason}, nil
}

type shelfRules struct {
	ignore shelfutil.IgnoreRules
	nsfw   shelfutil.NSFWRules
}

func loadShelfRules(root fsutil.ReadFS, logger logutil.Logger) shelfRules {
	conf, hasConfig := readShelfConfig(root, logger)
	return shelfRules{
		ignore: ignoreRulesFrom(conf, hasConfig, logger),
		nsfw:   nsfwRulesFrom(conf, hasConfig, logger),
	}
}

func nsfwRulesFrom(conf shelfConfigJSON, hasConfig bool, logger logutil.Logger) shelfutil.NSFWRules {
	if !hasConfig || len(conf.Content.NSFWFolders) == 0 {
		return shelfutil.NSFWRules{}
	}

	folders := make([]shelfutil.NSFWFolder, 0, len(conf.Content.NSFWFolders))
	for _, raw := range conf.Content.NSFWFolders {
		folder, err := parseNSFWFolder(raw)
		if err != nil {
			logger.Warn("ignoring an unreadable entry in the shelf configuration", "file", shelfConfigFile, "entry", string(raw), "error", err)
			continue
		}
		if err := shelfutil.ValidateNSFWFolderPath(folder.Path); err != nil {
			logger.Warn("ignoring an unusable entry in the shelf configuration", "file", shelfConfigFile, "path", folder.Path, "error", err)
			continue
		}
		folders = append(folders, folder)
	}

	if len(folders) == 0 {
		return shelfutil.NSFWRules{}
	}

	rules := shelfutil.NewNSFWRules(folders)
	logger.Info("shelf configuration marks folders as adult content",
		"file", shelfConfigFile, "nsfw_folders", rules.Paths())
	return rules
}

func ignoreRulesFrom(conf shelfConfigJSON, hasConfig bool, logger logutil.Logger) shelfutil.IgnoreRules {
	if !hasConfig || conf.Scan.IgnoredDirs == nil {
		return shelfutil.NewIgnoreRules(shelfutil.DefaultIgnoredDirs())
	}

	dirs := make([]shelfutil.IgnoredDir, 0, len(conf.Scan.IgnoredDirs))
	for _, raw := range conf.Scan.IgnoredDirs {
		dir, err := parseIgnoredDir(raw)
		if err != nil {
			logger.Warn("ignoring an unreadable entry in the shelf configuration", "file", shelfConfigFile, "entry", string(raw), "error", err)
			continue
		}
		if err := shelfutil.ValidatePathSegment(dir.Name); err != nil {
			logger.Warn("ignoring an unusable entry in the shelf configuration", "file", shelfConfigFile, "name", dir.Name, "error", err)
			continue
		}
		dirs = append(dirs, dir)
	}

	rules := shelfutil.NewIgnoreRules(dirs)
	logger.Info("shelf configuration replaces the directories skipped while scanning",
		"file", shelfConfigFile, "ignored_dirs", rules.Names())
	warnAboutDroppedDefaults(rules, logger)
	return rules
}

func warnAboutDroppedDefaults(rules shelfutil.IgnoreRules, logger logutil.Logger) {
	var dropped []string
	for _, dir := range shelfutil.DefaultIgnoredDirs() {
		if !rules.IsIgnoredDir(dir.Name) {
			dropped = append(dropped, dir.Name)
		}
	}
	if len(dropped) == 0 {
		return
	}

	logger.Warn("shelf configuration no longer skips directories PlainShelf skips by default; they will be listed as folders and any books inside them will appear in the library",
		"file", shelfConfigFile, "dropped", dropped)
}

func readShelfConfig(root fsutil.ReadFS, logger logutil.Logger) (shelfConfigJSON, bool) {
	file, err := root.Open(shelfConfigFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("ignoring an unreadable shelf configuration", "file", shelfConfigFile, "error", err)
		}
		return shelfConfigJSON{}, false
	}
	defer file.Close()

	if info, statErr := file.Stat(); statErr == nil && info.Size() > maxShelfConfigBytes {
		logger.Warn("ignoring a shelf configuration larger than this build reads",
			"file", shelfConfigFile, "size", info.Size(), "limit", maxShelfConfigBytes)
		return shelfConfigJSON{}, false
	}

	var conf shelfConfigJSON
	if err := json.UnmarshalRead(io.LimitReader(file, maxShelfConfigBytes), &conf); err != nil {
		logger.Warn("ignoring a shelf configuration that could not be read as JSON", "file", shelfConfigFile, "error", err)
		return shelfConfigJSON{}, false
	}

	if conf.SchemaVersion > shelfConfigSchemaVersion {
		logger.Warn("reading a shelf configuration written by a newer build; fields this build does not know are ignored",
			"file", shelfConfigFile, "schema_version", conf.SchemaVersion, "supported", shelfConfigSchemaVersion)
	}

	return conf, true
}

func (s *Shelf) ValidateFolderPath(folders FolderPath) error {
	if err := validateFolderPath(folders); err != nil {
		return util.Errorf("%w", err)
	}

	for _, folder := range folders {
		if reason, ignored := s.ignore.MatchIgnoredDir(folder); ignored {
			return &IgnoredFolderNameError{Folder: folder, Reason: reason}
		}
	}
	return nil
}
