package shelf

import (
	"io"
	"path"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// The affordances in this file exist for the one-off legacy source migration in
// internal/legacyupgrade. Ordinary shelf operations deliberately never change a
// legacy source's format ownership, nor erase a book's compatibility snapshots;
// see UpdateContent and SetMeta. Both are narrow on purpose, and both go away
// with the migration once no legacy shelves remain.

// ErrSourceNotLegacy is returned when a source already owns its content format.
var ErrSourceNotLegacy = util.NewError("source already owns its content format")

// IsLegacy reports whether this source predates source-level format metadata.
// Such a source inherits its format from book.json and its chapters from
// split_config; see SourceMetaSchemaVersion.
func (r *Source) IsLegacy() bool {
	return r.meta.SchemaVersion == 0 && r.meta.Format == ""
}

// UpgradeLegacyToSchemaV1 takes a legacy source into source metadata schema v1:
// it stamps the schema version and the now-authoritative format, and resets the
// split config the new schema ignores.
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

	rewritten := false
	if content != nil {
		sourceDestPath := path.Join(r.folderPath, SourceFile)
		if err := fsutil.WriteAtomic(r.root, sourceDestPath, content); err != nil {
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

	if err := r.writebackMeta(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// ClearLegacySourceFormats drops book.json's legacy_source_formats snapshots.
//
// The map only exists to restore the book-level format mirror when a legacy
// source is reactivated (see SetCurrentSource), so it is dead data once every
// source owns its own format. SetMeta deliberately refuses to let an ordinary
// metadata update erase it, so removal needs the internal setter; this is the
// whole-map counterpart to forgetLegacySourceFormat, which drops the single
// entry of a source being deleted.
//
// It drops the whole map unconditionally, including entries for sources that
// are still legacy. A book left with one is a book the migration could not
// finish, and the entry only ever decided which format the compatibility mirror
// took on reactivating that source; without it the mirror follows book.json's
// own format instead, which is the same fallback a shelf edited by hand gets.
func (b *Book) ClearLegacySourceFormats() error {
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	if len(b.meta.LegacySourceFormats) == 0 {
		// Nothing to drop. Returning here also keeps the book's file untouched,
		// rather than rewriting it only to stamp a schema version.
		return nil
	}

	meta := b.GetMeta()
	meta.LegacySourceFormats = nil
	// setMeta, not SetMeta: SetMeta clones the persisted map back over this.
	if err := b.setMeta(meta); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
