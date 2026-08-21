package shelf

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"path"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const SourceMetaFile = "meta.json"
const SourceFile = "source.txt"

// SourceMetaSchemaVersion is the source metadata format written for sources
// that own their content format. A missing version is a legacy source: its
// format and chapter behaviour are inherited from book.json and split_config.
const SourceMetaSchemaVersion = 1

var ErrUnsupportedSourceSchemaVersion = util.NewError("source meta.json schema version is newer than this build supports")

/*
{source-folder}/
├─ meta.json
└─ source.txt
*/

type Source struct {
	// root reads the shelf. It is a ReadFS so that a read path cannot mutate
	// the source by accident; every mutation narrows it back through writeRoot.
	root       fsutil.ReadFS
	folderPath string

	meta *SourceMeta
}

type SourceMeta struct {
	SchemaVersion int `json:"schema_version,omitempty"`

	ID        string        `json:"id"`
	CreatedAt util.JSONTime `json:"created_at"`
	Comment   string        `json:"comment"`
	Format    string        `json:"format,omitempty"`

	// depending on the content
	MD5Hash   string `json:"md5_hash,omitempty"`
	LineCount int    `json:"line_count,omitempty"`
	CharCount int    `json:"char_count,omitempty"`

	// split config: how the novel should be split into parts
	SplitConfig SplitConfig `json:"split_config"`
}

func (r *Source) FolderPath() string {
	return r.folderPath
}

func (r *Source) ID() string {
	return r.meta.ID
}

func (r *Source) GetMeta() *SourceMeta {
	meta := *r.meta
	meta.SplitConfig.Boundaries = append([]int(nil), r.meta.SplitConfig.Boundaries...)
	return &meta
}

// writeRoot returns the source's filesystem as a writable handle, reporting
// fsutil.ErrReadOnly when the source was opened on a read-only shelf.
func (r *Source) writeRoot() (fsutil.FS, error) {
	root, err := fsutil.Writable(r.root)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return root, nil
}

func (r *Source) EnsureWritable() error {
	if r.meta.SchemaVersion > SourceMetaSchemaVersion {
		return util.Errorf("%w: meta.json is schema_version %d, this build writes %d",
			ErrUnsupportedSourceSchemaVersion, r.meta.SchemaVersion, SourceMetaSchemaVersion)
	}
	return nil
}

func (r *Source) Open() (fs.File, error) {
	sourcePath := path.Join(r.folderPath, SourceFile)
	fp, err := r.root.Open(sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

// contentStat is the size and modification time of source.txt, which is what a
// cache keyed on "has this file changed" compares. Reported as an error rather
// than a zero value so a caller cannot mistake a missing source for an
// unchanged one.
func (r *Source) contentStat() (*FileStat, error) {
	stat, err := getFileStat(r.root, path.Join(r.folderPath, SourceFile))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return stat, nil
}

// readContent reads the whole source file. Every caller here needs more than
// one metric out of the same bytes, and reading once keeps that at a single
// round trip on a network-mounted shelf.
func (r *Source) readContent() ([]byte, error) {
	f, err := r.Open()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	data, readErr := io.ReadAll(f)
	closeErr := f.Close()
	if readErr != nil {
		return nil, util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return nil, util.Errorf("%w", closeErr)
	}

	return data, nil
}

func (r *Source) UpdateContent(newContent io.Reader) error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	sourceDestPath := path.Join(r.folderPath, SourceFile)

	if err := fsutil.WriteAtomic(root, sourceDestPath, newContent); err != nil {
		return util.Errorf("%w", err)
	}

	if err := r.refreshContentMetadata(); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (r *Source) VerifyContent() (bool, error) {
	sourceFile, err := r.Open()
	if err != nil {
		return false, util.Errorf("%w", err)
	}
	defer sourceFile.Close()

	md5Hash, err := hashutil.MD5Hash(sourceFile)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	return md5Hash == r.meta.MD5Hash, nil
}

func (r *Source) UpdateHash() error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	sourceFile, err := r.Open()
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer sourceFile.Close()

	r.meta.MD5Hash, err = hashutil.MD5Hash(sourceFile)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = r.writebackMeta(root)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (r *Source) RefreshContentMetadata() error {
	return r.refreshContentMetadata()
}

func (r *Source) refreshContentMetadata() error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	// Read the file once; compute all three metrics from the buffer to avoid
	// 3 separate SMB round-trips on network-mounted shelves.
	data, err := r.readContent()
	if err != nil {
		return util.Errorf("%w", err)
	}

	r.meta.MD5Hash, err = hashutil.MD5Hash(bytes.NewReader(data))
	if err != nil {
		return util.Errorf("%w", err)
	}

	r.meta.LineCount, err = util.LineCount(bytes.NewReader(data))
	if err != nil {
		return util.Errorf("%w", err)
	}

	r.meta.CharCount, err = util.CharCount(bytes.NewReader(data))
	if err != nil {
		return util.Errorf("%w", err)
	}

	return r.writebackMeta(root)
}

// repairContentHash rewrites meta.json's md5_hash when it disagrees with the
// content hash, reporting whether it wrote anything. A source edited outside
// PlainShelf carries a confidently wrong hash (nothing revalidates it after
// import); only a caller that just read the whole file knows the true value.
//
// Deliberately narrow: line_count and char_count are stale in the same case
// but are UI-visible, so a background task must not change them as a side
// effect; and a source with no md5_hash (a legacy import) is left alone, since
// filling it in would trigger the whole-library rewrite
// docs/concepts/data-format-versioning.md exists to avoid.
func (r *Source) repairContentHash(md5Hash string) (bool, error) {
	if md5Hash == "" || r.meta.MD5Hash == "" || r.meta.MD5Hash == md5Hash {
		return false, nil
	}
	if err := r.EnsureWritable(); err != nil {
		return false, util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	r.meta.MD5Hash = md5Hash
	if err := r.writebackMeta(root); err != nil {
		return false, util.Errorf("%w", err)
	}
	return true, nil
}

// UpdateComment replaces the source's free-form comment. It records how this
// source came to be — for example what an import could not carry over — and is
// rewritten whenever the content is imported again.
func (r *Source) UpdateComment(comment string) error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	r.meta.Comment = comment
	if err := r.writebackMeta(root); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// writebackMeta persists r.meta. It takes the writable handle rather than
// narrowing r.root itself: a caller has already changed r.meta in memory by the
// time it gets here, so refusing a read-only shelf at this point would leave
// GetMeta reporting a value that never reached disk.
func (r *Source) writebackMeta(root fsutil.FS) error {
	metaFilePath := path.Join(r.folderPath, SourceMetaFile)

	bs, err := json.MarshalIndent(r.meta, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}
	bs = append(bs, '\n')

	if err := fsutil.WriteFileAtomic(root, metaFilePath, bs); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func openSource(rt fsutil.ReadFS, sourcePath string) (*Source, error) {
	metaPath := path.Join(sourcePath, SourceMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta SourceMeta
	decoder := json.NewDecoder(metaFile)
	err = decoder.Decode(&meta)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Source{
		root:       rt,
		folderPath: sourcePath,
		meta:       &meta,
	}, nil
}

func createSource(rt fsutil.FS, logger logutil.Logger, sourcePath, id string, source io.Reader, format, comment string) (*Source, error) {
	if !validateBookFormat(format) || format == "" {
		return nil, util.Errorf("%w: got %q", ErrInvalidBookFormat, format)
	}
	err := rt.MkdirAll(sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourceDestPath := path.Join(sourcePath, SourceFile)
	if err := fsutil.WriteAtomic(rt, sourceDestPath, source); err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Read the written file once to compute all three metrics.
	destFile1, err := rt.Open(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	sourceData, readErr := io.ReadAll(destFile1)
	readCloseErr := destFile1.Close()
	if readErr != nil {
		return nil, util.Errorf("%w", readErr)
	}
	if readCloseErr != nil {
		return nil, util.Errorf("%w", readCloseErr)
	}

	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(sourceData))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	lineCount, err := util.LineCount(bytes.NewReader(sourceData))
	if err != nil {
		lineCount = 0
		logger.Error("failed to count lines", "error", err)
	}

	charCount, err := util.CharCount(bytes.NewReader(sourceData))
	if err != nil {
		charCount = 0
		logger.Error("failed to count characters", "error", err)
	}

	meta := SourceMeta{
		SchemaVersion: SourceMetaSchemaVersion,
		ID:            id,
		CreatedAt:     util.JSONTime(time.Now()),
		Format:        format,
		SplitConfig:   SplitConfig{Type: SplitTypeNone},

		MD5Hash:   md5Hash,
		LineCount: lineCount,
		CharCount: charCount,
		Comment:   comment,
	}

	metaFilePath := path.Join(sourcePath, SourceMetaFile)

	bs, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	bs = append(bs, '\n')

	if err := fsutil.WriteFileAtomic(rt, metaFilePath, bs); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Source{
		root:       rt,
		folderPath: sourcePath,
		meta:       &meta,
	}, nil
}
