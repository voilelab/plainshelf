package shelf

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

// shelfConfigFile is the shelf's own settings, sitting at the shelf root beside
// books/, trash/ and app/.
//
// It lives there rather than under app/ because app/ is documented as
// rebuildable runtime state a user may delete at will, and a setting that
// vanishes with the cache is worse than no setting at all. It travels with the
// shelf rather than with the server config so that every reader of the same
// shelf - this server, the desktop app, the Android client reading it from
// pCloud - applies the same rules, which a per-installation config file cannot
// guarantee.
//
// PlainShelf only ever reads it. Nothing in this build writes or rewrites it, so
// a hand-edited file keeps its formatting, its key order, and any key a later
// build adds.
const shelfConfigFile = "shelf.json"

// shelfConfigSchemaVersion is the shelf.json shape this build documents and
// reads. A file that declares a higher one is still read: every field this build
// understands is applied and the rest is left alone, which is safe precisely
// because reading is all that happens here.
const shelfConfigSchemaVersion = 1

// maxShelfConfigBytes bounds how much of shelf.json is read.
const maxShelfConfigBytes = 1 << 20

// shelfConfigJSON mirrors shelf.json. Unknown fields are accepted on purpose, so
// a shelf shared with a newer build is not rejected by an older one.
type shelfConfigJSON struct {
	SchemaVersion int `json:"schema_version"`
	Scan          struct {
		// ExtraIgnoredDirs are directory names under books/ this shelf skips on
		// top of the built-in list. The configuration can only add: the built-in
		// names stay ignored whatever the file says, because unignoring "@eaDir"
		// on a Synology share is how a library grows a duplicate folder tree.
		ExtraIgnoredDirs []string `json:"extra_ignored_dirs"`
	} `json:"scan"`
}

// loadIgnoreRules reads shelf.json and returns the directory-ignore rules for
// this shelf. The rules are read once, when the shelf is opened, and never
// change while it is open: every listing, every scan and every folder name
// checked against them within one run answers the same way. Editing shelf.json
// takes effect the next time the shelf is opened.
//
// No failure here stops a shelf from opening. A missing file is the normal case
// and means the built-in rules; an unreadable or malformed one is reported and
// then also means the built-in rules, because locking a user out of their
// library over a typo in an optional file would be a far worse outcome than
// listing a directory they wanted hidden.
func loadIgnoreRules(root fsutil.ReadFS, logger logutil.Logger) shelfutil.IgnoreRules {
	file, err := root.Open(shelfConfigFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("ignoring an unreadable shelf configuration", "file", shelfConfigFile, "error", err)
		}
		return shelfutil.IgnoreRules{}
	}
	defer file.Close() //nolint:errcheck // read-only

	var conf shelfConfigJSON
	// A settings file is a handful of lines; the limit is only there so a
	// mis-named multi-gigabyte file in the shelf root cannot be read into memory.
	if err := json.NewDecoder(io.LimitReader(file, maxShelfConfigBytes)).Decode(&conf); err != nil {
		logger.Warn("ignoring a shelf configuration that could not be read as JSON", "file", shelfConfigFile, "error", err)
		return shelfutil.IgnoreRules{}
	}

	if conf.SchemaVersion > shelfConfigSchemaVersion {
		logger.Warn("reading a shelf configuration written by a newer build; fields this build does not know are ignored",
			"file", shelfConfigFile, "schema_version", conf.SchemaVersion, "supported", shelfConfigSchemaVersion)
	}

	extra := make([]string, 0, len(conf.Scan.ExtraIgnoredDirs))
	for _, name := range conf.Scan.ExtraIgnoredDirs {
		if err := shelfutil.ValidatePathSegment(name); err != nil {
			if errors.Is(err, shelfutil.ErrIgnoredPathSegment) {
				// Already skipped by the built-in rules, so the entry is
				// redundant rather than wrong. Worth a line while debugging a
				// configuration, not a warning.
				logger.Debug("shelf configuration lists a directory that is already ignored", "file", shelfConfigFile, "name", name)
				continue
			}
			logger.Warn("ignoring an unusable entry in the shelf configuration", "file", shelfConfigFile, "name", name, "error", err)
			continue
		}
		extra = append(extra, name)
	}

	if len(extra) > 0 {
		logger.Info("shelf configuration adds directories to skip while scanning", "file", shelfConfigFile, "extra_ignored_dirs", extra)
	}

	return shelfutil.NewIgnoreRules(extra)
}

// ValidateFolderPath reports whether every segment of a folder path is safe to
// use on this shelf.
//
// It is the shelf-aware half of the check: the package-level rules reject the
// built-in system names, and this adds the names this shelf's own configuration
// skips. Creating a folder the scanners skip would appear to work and then
// vanish from the very next listing, so a configured name is refused for the
// same reason "@eaDir" is.
func (s *Shelf) ValidateFolderPath(folders FolderPath) error {
	if err := validateFolderPath(folders); err != nil {
		return util.Errorf("%w", err)
	}

	for _, folder := range folders {
		if s.ignore.IsExtraIgnoredDir(folder) {
			return util.Errorf("%w %q: %s lists it under scan.extra_ignored_dirs", ErrConfiguredIgnoredFolderName, folder, shelfConfigFile)
		}
	}
	return nil
}
