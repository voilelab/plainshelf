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

/*
{source-folder}/
├─ meta.json
└─ source.txt
*/

type Source struct {
	root       fsutil.FS
	folderPath string

	meta *SourceMeta
}

type SourceMeta struct {
	ID        string        `json:"id"`
	CreatedAt util.JSONTime `json:"created_at"`
	Comment   string        `json:"comment"`

	// depending on the content
	MD5Hash   string `json:"md5_hash,omitempty"`
	LineCount int    `json:"line_count,omitempty"`
	CharCount int    `json:"char_count,omitempty"`

	// split config: how the novel should be split into parts
	SplitConfig SplitConfig `json:"split_config,omitempty"`
}

func (r *Source) FolderPath() string {
	return r.folderPath
}

func (r *Source) ID() string {
	return r.meta.ID
}

func (r *Source) GetMeta() *SourceMeta {
	return r.meta
}

func (r *Source) Open() (fs.File, error) {
	sourcePath := path.Join(r.folderPath, SourceFile)
	fp, err := r.root.Open(sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (r *Source) UpdateContent(newContent io.Reader) error {
	sourceDestPath := path.Join(r.folderPath, SourceFile)
	destFile, err := r.root.OpenWriter(sourceDestPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	_, err = io.Copy(destFile, newContent)
	if err != nil {
		destFile.Close()
		return util.Errorf("%w", err)
	}
	if err := destFile.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	err = r.refreshContentMetadata()
	if err != nil {
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
	sourceFile, err := r.Open()
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer sourceFile.Close()

	r.meta.MD5Hash, err = hashutil.MD5Hash(sourceFile)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = r.writebackMeta()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (r *Source) refreshContentMetadata() error {
	// Read the file once; compute all three metrics from the buffer to avoid
	// 3 separate SMB round-trips on network-mounted shelves.
	f, err := r.Open()
	if err != nil {
		return util.Errorf("%w", err)
	}
	data, readErr := io.ReadAll(f)
	closeErr := f.Close()
	if readErr != nil {
		return util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return util.Errorf("%w", closeErr)
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

	return r.writebackMeta()
}

func (r *Source) withSourceFile(read func(fs.File) error) error {
	sourceFile, err := r.Open()
	if err != nil {
		return util.Errorf("%w", err)
	}

	readErr := read(sourceFile)
	closeErr := sourceFile.Close()
	if readErr != nil {
		return util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return util.Errorf("%w", closeErr)
	}
	return nil
}

func (r *Source) UpdateSplitConfig(config SplitConfig) error {
	r.meta.SplitConfig = config
	err := r.writebackMeta()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (r *Source) writebackMeta() error {
	metaFilePath := path.Join(r.folderPath, SourceMetaFile)
	metaFile, err := r.root.OpenWriter(metaFilePath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer metaFile.Close()

	encoder := json.NewEncoder(metaFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(r.meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func openSource(rt fsutil.FS, sourcePath string) (*Source, error) {
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

func createSource(rt fsutil.FS, logger logutil.Logger, sourcePath, id string, source io.Reader) (*Source, error) {
	err := rt.MkdirAll(sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourceDestPath := path.Join(sourcePath, SourceFile)
	destFile, err := rt.OpenWriter(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	_, err = io.Copy(destFile, source)
	if err != nil {
		destFile.Close()
		return nil, util.Errorf("%w", err)
	}
	destFile.Close()

	// Read the written file once to compute all three metrics.
	destFile1, err := rt.Open(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	sourceData, readErr := io.ReadAll(destFile1)
	closeErr := destFile1.Close()
	if readErr != nil {
		return nil, util.Errorf("%w", readErr)
	}
	if closeErr != nil {
		return nil, util.Errorf("%w", closeErr)
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
		ID:        id,
		CreatedAt: util.JSONTime(time.Now()),

		MD5Hash:   md5Hash,
		LineCount: lineCount,
		CharCount: charCount,
		Comment:   "",
	}

	metaFilePath := path.Join(sourcePath, SourceMetaFile)
	metaFile, err := rt.OpenWriter(metaFilePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	encoder := json.NewEncoder(metaFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(meta)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Source{
		root:       rt,
		folderPath: sourcePath,
		meta:       &meta,
	}, nil
}
