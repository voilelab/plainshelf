package bookpkg

import (
	"encoding/json/v2"
	"io"
	"io/fs"
	"path"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

const SourceMetaFile = "meta.json"
const SourceFile = "source.txt"

const SourceMetaSchemaVersion = 1

var ErrUnsupportedSourceSchemaVersion = util.NewError("source meta.json schema version is newer than this build supports")

type Source struct {
	root       fsutil.ReadFS
	folderPath string

	meta *SourceMeta
}

type SourceMeta struct {
	SchemaVersion int `json:"schema_version,omitzero"`

	ID        string        `json:"id"`
	CreatedAt util.JSONTime `json:"created_at"`
	Comment   string        `json:"comment"`
	Format    string        `json:"format,omitempty"`

	MD5Hash   string `json:"md5_hash,omitempty"`
	LineCount int    `json:"line_count,omitzero"`
	CharCount int    `json:"char_count,omitzero"`
}

func (r *Source) FolderPath() string {
	return r.folderPath
}

func (r *Source) ID() string {
	return r.meta.ID
}

func (r *Source) GetMeta() *SourceMeta {
	meta := *r.meta
	return &meta
}

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

func (r *Source) ContentStat() (*FileStat, error) {
	stat, err := getFileStat(r.root, path.Join(r.folderPath, SourceFile))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return stat, nil
}

func (r *Source) ReadContent() ([]byte, error) {
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
	f, err := r.Open()
	if err != nil {
		return util.Errorf("%w", err)
	}
	stats, readErr := measureContent(f)
	closeErr := f.Close()
	if readErr != nil {
		return util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return util.Errorf("%w", closeErr)
	}
	r.meta.MD5Hash, r.meta.LineCount, r.meta.CharCount = stats.md5Hash, stats.lineCount, stats.charCount

	return r.writebackMeta(root)
}

func (r *Source) RepairContentHash(md5Hash string) (bool, error) {
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

func (r *Source) writebackMeta(root fsutil.FS) error {
	metaFilePath := path.Join(r.folderPath, SourceMetaFile)

	bs, err := json.Marshal(r.meta, jsonopt.Disk())
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
	if err := json.UnmarshalRead(metaFile, &meta); err != nil {
		return nil, util.Errorf("%w", MetadataReadError(metaPath, err))
	}

	return &Source{
		root:       rt,
		folderPath: sourcePath,
		meta:       &meta,
	}, nil
}

func createSource(rt fsutil.FS, sourcePath, id string, source io.Reader, format, comment string) (*Source, error) {
	if !shelfutil.ValidateBookFormat(format) || format == "" {
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

	destFile, err := rt.Open(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	stats, readErr := measureContent(destFile)
	readCloseErr := destFile.Close()
	if readErr != nil {
		return nil, util.Errorf("%w", readErr)
	}
	if readCloseErr != nil {
		return nil, util.Errorf("%w", readCloseErr)
	}

	meta := SourceMeta{
		SchemaVersion: SourceMetaSchemaVersion,
		ID:            id,
		CreatedAt:     util.JSONTime(time.Now()),
		Format:        format,

		MD5Hash:   stats.md5Hash,
		LineCount: stats.lineCount,
		CharCount: stats.charCount,
		Comment:   comment,
	}

	metaFilePath := path.Join(sourcePath, SourceMetaFile)

	bs, err := json.Marshal(meta, jsonopt.Disk())
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

type contentStats struct {
	md5Hash   string
	lineCount int
	charCount int
}

func measureContent(r io.Reader) (contentStats, error) {
	var text util.TextCounter
	md5Hash, err := hashutil.MD5Hash(io.TeeReader(r, &text))
	if err != nil {
		return contentStats{}, util.Errorf("%w", err)
	}
	return contentStats{
		md5Hash:   md5Hash,
		lineCount: text.Lines(),
		charCount: text.Chars(),
	}, nil
}
