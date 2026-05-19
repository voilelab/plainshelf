package shelf

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
{book-folder}/
├─ book.json
├─ CURRENT_VERSION_LOCATION.txt
├─ cover.(jpg|png|webp)
└─ sources/
   └─ {source-id}
*/

const SourcesFolder = "sources"
const BookMetaFile = "book.json"
const CurrentVersionLocationFile = "CURRENT_VERSION_LOCATION.txt"
const CurrentVersionLocationTemplate = `[shelf 狀態指標]
當前閱讀版本存放於：
%s

(註：請勿修改此檔案內容，shelf 會自動更新此指標)
`

type Layers []string

func (l Layers) String() string {
	return strings.Join(l, "/")
}

func NewLayersFromString(s string) Layers {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

type Book struct {
	root       fsutil.FS
	folderPath string
	meta       *BookMeta
	layers     Layers
}

type BookMeta struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Format      string        `json:"format,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Cover       string        `json:"cover"`
	Authors     []string      `json:"authors"`
	Language    string        `json:"language"`
	Comments    string        `json:"comments"`
	CreatedAt   util.JSONTime `json:"created_at,omitzero"`
	UpdatedAt   util.JSONTime `json:"updated_at,omitzero"`
	PublishedAt util.JSONTime `json:"published_at,omitzero"`

	// User should not modify CurrentSource directly, it is managed by shelf internally,
	// and can be updated via SetCurrentSource method
	CurrentSource string `json:"current_source"`
}

// setLayers only used for internal use, not persisted in book meta, and not exposed to user
func (b *Book) setLayers(layers Layers) {
	b.layers = layers
}

func (b *Book) Layers() Layers {
	return b.layers
}

func (b *Book) ID() string {
	return b.meta.ID
}

func (b *Book) Title() string {
	return b.meta.Title
}

func (b *Book) FolderPath() string {
	return b.folderPath
}

func (b *Book) OpenCover() ([]byte, string, error) {
	if b.meta.Cover == "" {
		return nil, "", nil
	}

	coverPath := path.Join(b.folderPath, b.meta.Cover)
	coverFile, err := b.root.Open(coverPath)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	defer coverFile.Close()

	coverData, err := io.ReadAll(coverFile)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}

	ext := path.Ext(b.meta.Cover)
	return coverData, ext, nil
}

func (b *Book) SetCover(imageData []byte, ext string) error {
	coverFilename := "cover" + ext
	coverPath := path.Join(b.folderPath, coverFilename)

	coverFile, err := b.root.OpenWriter(coverPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer coverFile.Close()

	_, err = coverFile.Write(imageData)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = coverFilename
	err = b.SetMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) DeleteCover() error {
	if b.meta.Cover == "" {
		return nil
	}

	coverPath := path.Join(b.folderPath, b.meta.Cover)
	err := b.root.Remove(coverPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = ""
	err = b.setMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) CurrentSource() string {
	return b.meta.CurrentSource
}

func (b *Book) SetCurrentSource(sourceID string) error {
	meta := b.GetMeta()
	meta.CurrentSource = sourceID

	err := b.setMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = b.updateCurrentVersionLocation(sourceID)
	if err != nil {
		// TBD: rollback meta update?
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) updateCurrentVersionLocation(sourceID string) error {
	sourcePath := path.Join(SourcesFolder, sourceID, SourceFile)
	sourceContent := fmt.Sprintf(CurrentVersionLocationTemplate, sourcePath)

	fp, err := b.root.OpenWriter(path.Join(b.folderPath, CurrentVersionLocationFile))
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer fp.Close()

	n, err := fp.Write([]byte(sourceContent))
	if err != nil {
		return util.Errorf("%w", err)
	}

	if n != len(sourceContent) {
		return util.Errorf("incomplete write: expected %d bytes, wrote %d bytes", len(sourceContent), n)
	}

	return nil
}

// GetMeta returns a copy of the book meta, user can modify the returned meta and call SetMeta to update the book meta, but should not modify the CurrentSource field directly
func (b *Book) GetMeta() *BookMeta {
	metaCopy := *b.meta
	metaCopy.Tags = append([]string(nil), b.meta.Tags...)
	metaCopy.Authors = append([]string(nil), b.meta.Authors...)
	return &metaCopy
}

// SetMeta allows user to update book meta, but not the CurrentSource field which is managed by shelf internally
func (b *Book) SetMeta(meta *BookMeta) error {
	if meta.CurrentSource != b.meta.CurrentSource {
		return util.NewError("cannot modify CurrentSource field directly, use SetCurrentSource method instead")
	}

	return b.setMeta(meta)
}

func (b *Book) setMeta(meta *BookMeta) error {
	if meta == nil {
		return util.NewError("meta cannot be nil")
	}

	if !validateBCP47(meta.Language) {
		return util.Errorf("invalid language tag: %s", meta.Language)
	}

	// write back to book meta

	encoder := json.NewEncoder(io.Discard)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(meta); err != nil {
		return util.Errorf("%w", err)
	}

	metaPath := path.Join(b.folderPath, BookMetaFile)

	// TBD: write to a temp file and rename to ensure atomic update
	metaFile, err := b.root.OpenWriter(metaPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer metaFile.Close()

	encoder = json.NewEncoder(metaFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	b.meta = meta
	return nil
}

func (b *Book) NewSource(source io.Reader) (*Source, error) {
	// create a new source for the given book with the provided source file and metadata
	// TBD: atomic operation, rollback on failure
	sourceID := time.Now().Format("20060102-150405")
	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)

	src, err := createSource(b.root, sourcePath, sourceID, source)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	err = b.updateCurrentVersionLocation(sourceID)
	if err != nil {
		// TBD: rollback meta update?
		return nil, util.Errorf("%w", err)
	}

	return src, nil
}

func (b *Book) GetSource(sourceID string) (*Source, error) {
	if err := validateSourceID(sourceID); err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)
	source, err := openSource(b.root, sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return source, nil
}

func (b *Book) ListSource() ([]*Source, error) {
	sourcesPath := path.Join(b.folderPath, SourcesFolder)

	sourceEntries, err := b.root.ReadDir(sourcesPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var sources []*Source
	for _, entry := range sourceEntries {
		revID := entry.Name()
		sourcePath := path.Join(sourcesPath, revID)
		source, err := openSource(b.root, sourcePath)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}

		sources = append(sources, source)
	}

	return sources, nil
}

func openBook(rt fsutil.FS, bookPath string) (*Book, error) {
	bookFolder, err := rt.Stat(bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if !bookFolder.IsDir() {
		return nil, util.Errorf("%s is not a book directory", bookPath)
	}

	metaPath := path.Join(bookPath, BookMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta BookMeta
	decoder := json.NewDecoder(metaFile)
	if err := decoder.Decode(&meta); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       &meta,
	}, nil
}

func createBook(rt fsutil.FS, bookPath, bookID, title string) (*Book, error) {
	err := rt.MkdirAll(bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	meta := BookMeta{
		ID:        bookID,
		Title:     title,
		CreatedAt: util.JSONTime(time.Now()),
	}

	metaFilePath := path.Join(bookPath, BookMetaFile)
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

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       &meta,
	}, nil
}
