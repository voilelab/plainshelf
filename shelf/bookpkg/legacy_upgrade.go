package bookpkg

import (
	"io"
	"path"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// The affordances in this file exist for the one-off legacy source migration in
// internal/legacyupgrade. Ordinary shelf operations deliberately never change a
// legacy source's format ownership; see UpdateContent. They are narrow on
// purpose, and go away with the migration once no legacy shelves remain.

// ErrSourceNotLegacy is returned when a source already owns its content format.
var ErrSourceNotLegacy = util.NewError("source already owns its content format")

// IsLegacy reports whether this source predates source-level format metadata.
// Such a source inherits its format from book.json and its chapters from
// split_config; see SourceMetaSchemaVersion.
func (r *Source) IsLegacy() bool {
	return r.meta.SchemaVersion == 0 && r.meta.Format == ""
}

// UpgradeLegacyToSchemaV1 takes a legacy source into source metadata schema v1:
// it stamps the schema version and the now-authoritative format, and clears the
// split config the new schema ignores. Clearing it to the zero value drops the
// split_config key from meta.json entirely, thanks to its omitzero tag.
//
// Pass a nil content to stamp metadata only, leaving source.txt untouched.
// Otherwise content replaces the text — that is how a legacy split configuration
// becomes Markdown headings.
//
// The text is written before the metadata, and that order matters. Interrupted
// after the text, the source has headings but still reads as legacy: its
// chapters look wrong until the migration runs again, which then sees the
// headings and only needs the metadata. Interrupted the other way, meta.json
// would claim a format the text does not carry and the source would no longer
// look legacy, so nothing would ever repair it.
func (r *Source) UpgradeLegacyToSchemaV1(format string, content io.Reader) error {
	// Guard before the write, not just before the metadata: a source this build
	// must not touch should not have its text replaced either.
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	if !r.IsLegacy() {
		return util.Errorf("%w: %q", ErrSourceNotLegacy, r.meta.ID)
	}
	if format != BookFormatText && format != BookFormatMarkdown {
		return util.Errorf("%w: got %q", ErrInvalidBookFormat, format)
	}
	// Narrowed before the in-memory stamp below, not just before the write:
	// a refused upgrade must not leave this Source claiming a schema version
	// and format that never reached meta.json.
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	rewritten := false
	if content != nil {
		sourceDestPath := path.Join(r.folderPath, SourceFile)
		if err := fsutil.WriteAtomic(root, sourceDestPath, content); err != nil {
			return util.Errorf("%w", err)
		}
		rewritten = true
	}

	r.meta.SchemaVersion = SourceMetaSchemaVersion
	r.meta.Format = format
	r.meta.SplitConfig = SplitConfig{Type: SplitTypeNone}

	if rewritten {
		// Recomputes the content metrics from the new bytes and writes meta.json
		// once, so the stamp and the fresh hash land together.
		if err := r.refreshContentMetadata(); err != nil {
			return util.Errorf("%w", err)
		}
		return nil
	}

	if err := r.writebackMeta(root); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
