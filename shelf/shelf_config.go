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

// shelfConfigFile is the shelf's own settings, at the shelf root beside books/,
// trash/ and app/.
//
// Not under app/, which is documented as rebuildable state a user may delete at
// will; and travelling with the shelf rather than with the server config, so
// every reader of it — this server, the desktop app, the Android client on
// pCloud — applies the same rules.
//
// PlainShelf only ever reads it, so a hand-edited file keeps its formatting, its
// key order, and any key a later build adds.
const shelfConfigFile = "shelf.json"

// shelfConfigSchemaVersion is the shelf.json shape this build documents and
// reads. A file that declares a higher one is still read: every field this build
// understands is applied and the rest is left alone, which is safe precisely
// because reading is all that happens here.
const shelfConfigSchemaVersion = 1

// maxShelfConfigBytes bounds how large a shelf.json this build reads. A settings
// file is a handful of lines; the limit is there so a mis-named large file in
// the shelf root is skipped rather than read into memory. The pCloud client
// applies the same limit to the same file, from the size in its listing.
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
		// knows what their own folders hold. A folder has no metadata file of its
		// own (docs/concepts/folders.md), so this is the only place a
		// folder-level mark can live.
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

// parseIgnoredDir reads one entry. A bare name is not accepted: "@eaDir" and
// {"name": "@eaDir"} would mean the same thing and the file would have two
// shapes for one entry, which every reader of it then has to handle.
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

// parseNSFWFolder reads one entry. As with parseIgnoredDir, a bare path is not
// accepted: one entry, one shape.
func parseNSFWFolder(raw jsontext.Value) (shelfutil.NSFWFolder, error) {
	var object nsfwFolderJSON
	if err := json.Unmarshal(raw, &object); err != nil {
		return shelfutil.NSFWFolder{}, util.Errorf("entry is not a {path, reason} object: %w", err)
	}
	return shelfutil.NSFWFolder{Path: object.Path, Reason: object.Reason}, nil
}

// shelfRules is everything this build reads out of shelf.json. It is read once,
// when the shelf is opened, and never written again, so every listing, scan and
// folder check within one run answers the same way. Editing shelf.json takes
// effect the next time the shelf is opened.
type shelfRules struct {
	ignore shelfutil.IgnoreRules
	nsfw   shelfutil.NSFWRules
}

// loadShelfRules reads shelf.json once and derives every rule set from it.
//
// No failure here stops a shelf from opening. A missing file is the normal case;
// an unreadable or malformed one is reported and then means the same thing,
// because locking a user out of their library over a typo in an optional file
// would be a far worse outcome than listing a directory they wanted hidden.
func loadShelfRules(root fsutil.ReadFS, logger logutil.Logger) shelfRules {
	conf, hasConfig := readShelfConfig(root, logger)
	return shelfRules{
		ignore: ignoreRulesFrom(conf, hasConfig, logger),
		nsfw:   nsfwRulesFrom(conf, hasConfig, logger),
	}
}

// nsfwRulesFrom builds the content rules from a shelf.json this build has
// already read. An entry that cannot name a folder is dropped one by one, like
// an unusable ignored_dirs entry: the rest of the list is still what the shelf
// said.
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
		// Every entry was dropped, so this shelf marks nothing and there is no
		// list worth logging.
		return shelfutil.NSFWRules{}
	}

	rules := shelfutil.NewNSFWRules(folders)
	logger.Info("shelf configuration marks folders as adult content",
		"file", shelfConfigFile, "nsfw_folders", rules.Paths())
	return rules
}

// ignoreRulesFrom builds the directory-ignore rules from a shelf.json this build
// has already read.
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

// warnAboutDroppedDefaults reports the built-in names a configured shelf no
// longer skips.
//
// The configuration wins - a user who lists their own directories has said what
// this shelf holds, and this build does not second-guess them. But dropping
// "@eaDir" on a Synology share is how a library quietly grows a duplicate folder
// tree, and dropping "#recycle" is how deleted books come back, so the one place
// that can connect the two says it out loud rather than leaving the user to
// wonder where the folders came from.
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

// readShelfConfig reads and decodes shelf.json. The second result is false when
// the shelf has no usable configuration, whatever the reason - absent,
// oversized, unreadable - because every one of those means the same thing to the
// caller.
func readShelfConfig(root fsutil.ReadFS, logger logutil.Logger) (shelfConfigJSON, bool) {
	file, err := root.Open(shelfConfigFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("ignoring an unreadable shelf configuration", "file", shelfConfigFile, "error", err)
		}
		return shelfConfigJSON{}, false
	}
	defer file.Close() //nolint:errcheck // read-only

	if info, statErr := file.Stat(); statErr == nil && info.Size() > maxShelfConfigBytes {
		logger.Warn("ignoring a shelf configuration larger than this build reads",
			"file", shelfConfigFile, "size", info.Size(), "limit", maxShelfConfigBytes)
		return shelfConfigJSON{}, false
	}

	var conf shelfConfigJSON
	// The limit is applied again here for a file that grew since the stat above.
	//
	// UnmarshalRead consumes the whole reader and rejects anything after the
	// first value - a second object, or the debris of a half-finished edit -
	// which the pCloud reader also does by parsing the file in one go. Member
	// names are matched case-sensitively and a duplicate member is an error,
	// the same strictness book.json is read with.
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

// ValidateFolderPath reports whether every segment of a folder path is safe to
// use on this shelf.
//
// A name this shelf's scanners skip is refused here rather than by the
// package-level rules, which describe a path segment and cannot know what this
// shelf skips. Creating such a folder would appear to work and then vanish from
// the very next listing, so it is refused with the reason the name is skipped -
// which, on a shelf that lists its own directories, is a reason the user wrote.
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
