package txtlib

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"path"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const SnapshotMetaFile = "meta.json"
const SourceFile = "source.txt"

/*
{snapshot-folder}/
├─ meta.json
└─ text.txt
*/

type Snapshot struct {
	root       fsutil.FS
	folderPath string

	meta *SnapshotMeta
}

type SnapshotMeta struct {
	ID        string        `json:"id"`
	CreatedAt util.JSONTime `json:"created_at"`

	// depending on the content
	ContentHash string `json:"content_hash"`
	LineCount   int    `json:"line_count,omitempty"`
	CharCount   int    `json:"char_count,omitempty"`

	// recommend type: file, telegram, website, manual, generated, unknown
	SourceType  string `json:"source_type"`
	SourceLabel string `json:"source_label"`
	SourceURI   string `json:"source_uri"`
	Comment     string `json:"comment"`

	// split config: how the novel should be split into parts
	SplitConfig SplitConfig `json:"split_config,omitempty"`
}

func (r *Snapshot) FolderPath() string {
	return r.folderPath
}

func (r *Snapshot) ID() string {
	return r.meta.ID
}

func (r *Snapshot) GetMeta() *SnapshotMeta {
	return r.meta
}

func (r *Snapshot) OpenSource() (fs.File, error) {
	sourcePath := path.Join(r.folderPath, SourceFile)
	fp, err := r.root.Open(sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (r *Snapshot) VerifyContent() (bool, error) {
	sourceFile, err := r.OpenSource()
	if err != nil {
		return false, util.Errorf("%w", err)
	}
	defer sourceFile.Close()

	content, err := io.ReadAll(sourceFile)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	ok, err := hashutil.VerifyContentHash(content, r.meta.ContentHash)
	if err != nil {
		return false, util.Errorf("%w", err)
	}
	return ok, nil
}

func (r *Snapshot) UpdateHash() error {
	sourceFile, err := r.OpenSource()
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer sourceFile.Close()

	content, err := io.ReadAll(sourceFile)
	if err != nil {
		return util.Errorf("%w", err)
	}

	newHash, err := hashutil.NewContentHash(content)
	if err != nil {
		return util.Errorf("%w", err)
	}

	r.meta.ContentHash = newHash
	err = r.writebackMeta()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (r *Snapshot) UpdateSplitConfig(config SplitConfig) error {
	r.meta.SplitConfig = config
	err := r.writebackMeta()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (r *Snapshot) writebackMeta() error {
	metaFilePath := path.Join(r.folderPath, SnapshotMetaFile)
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

func openSnapshot(rt fsutil.FS, snapshotPath string) (*Snapshot, error) {
	metaPath := path.Join(snapshotPath, SnapshotMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta SnapshotMeta
	decoder := json.NewDecoder(metaFile)
	err = decoder.Decode(&meta)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Snapshot{
		root:       rt,
		folderPath: snapshotPath,
		meta:       &meta,
	}, nil
}

func createSnapshot(rt fsutil.FS, snapshotPath, id string, source io.Reader, sourceType, sourceLabel, sourceURI string) (*Snapshot, error) {
	err := rt.MkdirAll(snapshotPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourceDestPath := path.Join(snapshotPath, SourceFile)
	destFile, err := rt.OpenWriter(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer destFile.Close()

	var sourceContent bytes.Buffer
	_, err = io.Copy(io.MultiWriter(destFile, &sourceContent), source)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	contentHash, err := hashutil.NewContentHash(sourceContent.Bytes())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	destFile2, err := rt.Open(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer destFile2.Close()

	lineCount, err := util.LineCount(destFile2)
	if err != nil {
		lineCount = 0
		log.Println("failed to count lines:", err)
	}

	destFile3, err := rt.Open(sourceDestPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer destFile3.Close()

	charCount, err := util.CharCount(destFile3)
	if err != nil {
		charCount = 0
		log.Println("failed to count characters:", err)
	}

	meta := SnapshotMeta{
		ID:        id,
		CreatedAt: util.JSONTime(time.Now()),

		ContentHash: contentHash,
		LineCount:   lineCount,
		CharCount:   charCount,

		SourceType:  sourceType,
		SourceLabel: sourceLabel,
		SourceURI:   sourceURI,
		Comment:     "",
	}

	metaFilePath := path.Join(snapshotPath, SnapshotMetaFile)
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

	return &Snapshot{
		root:       rt,
		folderPath: snapshotPath,
		meta:       &meta,
	}, nil
}
