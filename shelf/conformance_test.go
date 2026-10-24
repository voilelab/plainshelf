package shelf

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/json/v2"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

const conformanceDatasetVersion = 4

const conformanceRoot = "testdata/conformance"

type conformanceManifest struct {
	SchemaVersion int `json:"schema_version"`
	Cases         []struct {
		Name   string `json:"name"`
		Covers string `json:"covers"`
	} `json:"cases"`
}

type conformanceReading struct {
	Folders    []string                         `json:"folders"`
	FolderMeta map[string]conformanceFolderMeta `json:"folder_meta"`
	Books      []conformanceBook                `json:"books"`
	BookCaches []conformanceBookCache           `json:"book_caches"`
}

type conformanceFolderMeta struct {
	SortKey    *float64 `json:"sort_key,omitzero"`
	Icon       string   `json:"icon"`
	IconOnDisk string   `json:"icon_on_disk"`
	ReadOnly   bool     `json:"read_only"`
}

type conformanceBook struct {
	Path                string              `json:"path"`
	Folders             []string            `json:"folders"`
	ID                  string              `json:"id"`
	Title               string              `json:"title"`
	Format              string              `json:"format"`
	Authors             []string            `json:"authors"`
	Tags                []string            `json:"tags"`
	Identifiers         map[string]string   `json:"identifiers"`
	Language            string              `json:"language"`
	Comments            string              `json:"comments"`
	Star                int                 `json:"star"`
	CreatedAt           string              `json:"created_at"`
	UpdatedAt           string              `json:"updated_at"`
	PublishedAt         string              `json:"published_at"`
	Cover               string              `json:"cover"`
	CoverPresent        bool                `json:"cover_present"`
	SchemaVersionOnDisk int                 `json:"schema_version_on_disk"`
	ReadOnly            bool                `json:"read_only"`
	NSFW                bool                `json:"nsfw"`
	CurrentSourceField  string              `json:"current_source_field"`
	CurrentSource       *string             `json:"current_source"`
	Sources             []conformanceSource `json:"sources"`
}

type conformanceSource struct {
	ID            string   `json:"id"`
	SchemaVersion int      `json:"schema_version"`
	CreatedAt     string   `json:"created_at"`
	Comment       string   `json:"comment"`
	Format        string   `json:"format"`
	MD5Hash       string   `json:"md5_hash"`
	HasContent    bool     `json:"has_content"`
	LineCount     int      `json:"line_count"`
	CharCount     int      `json:"char_count"`
	Assets        []string `json:"assets"`
}

type conformanceBookCache struct {
	Name    string   `json:"name"`
	Usable  bool     `json:"usable"`
	BookIDs []string `json:"book_ids"`
}

func TestBookPackageConformance(t *testing.T) {
	manifest := loadConformanceManifest(t)
	for _, entry := range manifest.Cases {
		t.Run(entry.Name, func(t *testing.T) {
			caseDir := filepath.Join(conformanceRoot, "cases", entry.Name)
			expected := loadConformanceExpectation(t, filepath.Join(caseDir, "expected.json"))
			observed := readConformanceCase(t, filepath.Join(caseDir, "shelf"))
			assertConformanceEqual(t, expected, observed)
		})
	}
}

func TestBookPackageConformanceThroughReadFS(t *testing.T) {
	manifest := loadConformanceManifest(t)
	for _, entry := range manifest.Cases {
		t.Run(entry.Name, func(t *testing.T) {
			caseDir := filepath.Join(conformanceRoot, "cases", entry.Name)
			expected := loadConformanceExpectation(t, filepath.Join(caseDir, "expected.json"))
			observed := readConformanceCaseThroughReadFS(t, filepath.Join(caseDir, "shelf"))
			assertConformanceEqual(t, expected, observed)
		})
	}
}

func TestBookPackageConformanceThroughPCloud(t *testing.T) {
	manifest := loadConformanceManifest(t)
	for _, entry := range manifest.Cases {
		t.Run(entry.Name, func(t *testing.T) {
			caseDir := filepath.Join(conformanceRoot, "cases", entry.Name)
			expected := loadConformanceExpectation(t, filepath.Join(caseDir, "expected.json"))
			s, _ := openPCloudShelf(t, filepath.Join(caseDir, "shelf"), ShelfConf{})
			observed := conformanceReading{
				Folders:    s.collectExportFolders(),
				FolderMeta: readConformanceFolderMeta(t, s),
				Books:      readConformanceBooks(t, s),
				BookCaches: readConformanceBookCaches(t, s),
			}
			assertConformanceEqual(t, expected, observed)
		})
	}
}

func TestBookPackageConformanceThroughWebDAV(t *testing.T) {
	manifest := loadConformanceManifest(t)
	for _, entry := range manifest.Cases {
		t.Run(entry.Name, func(t *testing.T) {
			caseDir := filepath.Join(conformanceRoot, "cases", entry.Name)
			expected := loadConformanceExpectation(t, filepath.Join(caseDir, "expected.json"))
			s := openWebDAVShelf(t, filepath.Join(caseDir, "shelf"))
			observed := conformanceReading{
				Folders:    s.collectExportFolders(),
				FolderMeta: readConformanceFolderMeta(t, s),
				Books:      readConformanceBooks(t, s),
				BookCaches: readConformanceBookCaches(t, s),
			}
			assertConformanceEqual(t, expected, observed)
		})
	}
}

func TestConformanceDatasetIsComplete(t *testing.T) {
	manifest := loadConformanceManifest(t)

	listed := make([]string, 0, len(manifest.Cases))
	for _, entry := range manifest.Cases {
		if entry.Covers == "" {
			t.Errorf("case %q has no \"covers\" line; say what it is for", entry.Name)
		}
		listed = append(listed, entry.Name)
	}
	slices.Sort(listed)

	entries, err := os.ReadDir(filepath.Join(conformanceRoot, "cases"))
	if err != nil {
		t.Fatalf("failed to read the conformance cases: %v", err)
	}
	var present []string
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf("unexpected file %q beside the conformance cases", entry.Name())
			continue
		}
		present = append(present, entry.Name())
	}
	slices.Sort(present)

	if !slices.Equal(listed, present) {
		t.Errorf("manifest.json lists %v but the cases directory holds %v", listed, present)
	}

	for _, name := range present {
		children, err := os.ReadDir(filepath.Join(conformanceRoot, "cases", name))
		if err != nil {
			t.Fatalf("failed to read case %q: %v", name, err)
		}
		var got []string
		for _, child := range children {
			got = append(got, child.Name())
		}
		slices.Sort(got)
		if !slices.Equal(got, []string{"expected.json", "shelf"}) {
			t.Errorf("case %q holds %v, want exactly expected.json and shelf/", name, got)
		}
	}
}

func loadConformanceManifest(t *testing.T) conformanceManifest {
	t.Helper()

	var manifest conformanceManifest
	decodeConformanceJSON(t, filepath.Join(conformanceRoot, "manifest.json"), &manifest)

	if manifest.SchemaVersion != conformanceDatasetVersion {
		t.Fatalf("conformance dataset is schema_version %d, this harness reads %d",
			manifest.SchemaVersion, conformanceDatasetVersion)
	}
	if len(manifest.Cases) == 0 {
		t.Fatal("conformance dataset lists no cases")
	}
	return manifest
}

func loadConformanceExpectation(t *testing.T, filePath string) conformanceReading {
	t.Helper()

	var expected conformanceReading
	decodeConformanceJSON(t, filePath, &expected)
	return expected
}

func decodeConformanceJSON(t *testing.T, filePath string, target any) {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filePath, err)
	}
	defer file.Close()

	if err := json.UnmarshalRead(file, target, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("failed to decode %s: %v", filePath, err)
	}
}

func readConformanceCase(t *testing.T, shelfDir string) conformanceReading {
	t.Helper()

	libRoot := filepath.Join(t.TempDir(), "shelf")
	if err := os.CopyFS(libRoot, os.DirFS(shelfDir)); err != nil {
		t.Fatalf("failed to copy the case shelf: %v", err)
	}

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	return conformanceReading{
		Folders:    s.collectExportFolders(),
		FolderMeta: readConformanceFolderMeta(t, s),
		Books:      readConformanceBooks(t, s),
		BookCaches: readConformanceBookCaches(t, s),
	}
}

func readConformanceCaseThroughReadFS(t *testing.T, shelfDir string) conformanceReading {
	t.Helper()

	rt, err := os.OpenRoot(shelfDir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })

	s, err := NewShelfFromFS(&ShelfConf{}, fsutil.NewRootFS(rt), "")
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	return conformanceReading{
		Folders:    s.collectExportFolders(),
		FolderMeta: readConformanceFolderMeta(t, s),
		Books:      readConformanceBooks(t, s),
		BookCaches: readConformanceBookCaches(t, s),
	}
}

func readOnDiskSchemaVersion(t *testing.T, s *Shelf, bookPath string) int {
	t.Helper()

	file, err := s.dbRoot.Open(path.Join(bookPath, BookMetaFile))
	if err != nil {
		t.Fatalf("open book.json (%s): %v", bookPath, err)
	}
	defer file.Close()

	var meta struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.UnmarshalRead(file, &meta); err != nil {
		t.Fatalf("decode book.json (%s): %v", bookPath, err)
	}
	return meta.SchemaVersion
}

func readConformanceFolderMeta(t *testing.T, s *Shelf) map[string]conformanceFolderMeta {
	t.Helper()

	metas, err := s.ListFolderMeta()
	if err != nil {
		t.Fatalf("ListFolderMeta: %v", err)
	}
	reading := map[string]conformanceFolderMeta{}
	for name, meta := range metas {
		var key *float64
		if meta.HasSortKey {
			value := meta.SortKey
			key = &value
		}
		reading[name] = conformanceFolderMeta{
			Icon:       meta.DisplayIcon(),
			IconOnDisk: meta.Icon,
			SortKey:    key,
			ReadOnly:   meta.ReadOnly,
		}
	}
	return reading
}

func readConformanceBooks(t *testing.T, s *Shelf) []conformanceBook {
	t.Helper()

	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}

	observed := make([]conformanceBook, 0, len(listings))
	for _, listing := range listings {
		book := listing.Book
		meta := book.GetMeta()

		onDiskSchema := readOnDiskSchemaVersion(t, s, book.PackagePath())

		coverPresent := false
		if meta.Cover != "" {
			if _, _, err := book.OpenCover(); err == nil {
				coverPresent = true
			}
		}

		var currentSource *string
		if source, err := book.ResolveCurrentSource(); err == nil {
			id := source.ID()
			currentSource = &id
		} else if !errors.Is(err, ErrSourceNotFound) {
			t.Fatalf("ResolveCurrentSource(%s): %v", book.PackagePath(), err)
		}

		observed = append(observed, conformanceBook{
			Path:                book.PackagePath(),
			Folders:             orEmpty(listing.Folders),
			ID:                  book.ID(),
			Title:               book.Title(),
			Format:              meta.Format,
			Authors:             orEmpty(meta.Authors),
			Tags:                orEmpty(meta.Tags),
			Identifiers:         orEmptyMap(meta.Identifiers),
			Language:            meta.Language,
			Comments:            meta.Comments,
			Star:                meta.Star,
			CreatedAt:           formatConformanceTime(meta.CreatedAt),
			UpdatedAt:           formatConformanceTime(meta.UpdatedAt),
			PublishedAt:         formatConformanceDate(meta.PublishedAt),
			Cover:               meta.Cover,
			CoverPresent:        coverPresent,
			SchemaVersionOnDisk: onDiskSchema,
			ReadOnly:            errors.Is(book.EnsureWritable(), ErrUnsupportedBookSchemaVersion),
			NSFW:                s.IsBookNSFW(listing.Folders, meta),
			CurrentSourceField:  book.CurrentSource(),
			CurrentSource:       currentSource,
			Sources:             readConformanceSources(t, s, book),
		})
	}

	slices.SortFunc(observed, func(a, b conformanceBook) int { return cmp.Compare(a.Path, b.Path) })
	return observed
}

func readConformanceSources(t *testing.T, s *Shelf, book *Book) []conformanceSource {
	t.Helper()

	sources, err := book.ListSource()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []conformanceSource{}
		}
		t.Fatalf("ListSource(%s): %v", book.PackagePath(), err)
	}

	observed := make([]conformanceSource, 0, len(sources))
	for _, source := range sources {
		hasContent := false
		if content, err := source.Open(); err == nil {
			hasContent = true
			content.Close()
		}

		meta := source.GetMeta()
		observed = append(observed, conformanceSource{
			ID:            source.ID(),
			SchemaVersion: meta.SchemaVersion,
			CreatedAt:     formatConformanceTime(meta.CreatedAt),
			Comment:       meta.Comment,
			Format:        meta.Format,
			MD5Hash:       meta.MD5Hash,
			HasContent:    hasContent,
			LineCount:     meta.LineCount,
			CharCount:     meta.CharCount,
			Assets:        readConformanceAssets(t, s, source),
		})
	}
	return observed
}

func readConformanceAssets(t *testing.T, s *Shelf, source *Source) []string {
	t.Helper()

	assetsPath := path.Join(source.FolderPath(), SourceAssetsFolder)
	entries, err := s.dbRoot.ReadDir(assetsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}
		}
		t.Fatalf("ReadDir(%s): %v", assetsPath, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		names = append(names, name)

		assetPath, err := source.AssetPath(name)
		if err != nil {
			t.Errorf("AssetPath(%s): %v", name, err)
			continue
		}
		if want := path.Join(assetsPath, name); assetPath != want {
			t.Errorf("AssetPath(%s) = %q, want %q", name, assetPath, want)
		}
		asset, err := source.OpenAsset(name)
		if err != nil {
			t.Errorf("OpenAsset(%s): %v", name, err)
			continue
		}
		asset.File.Close()
	}

	slices.Sort(names)
	var bundle bytes.Buffer
	zw := zip.NewWriter(&bundle)
	if err := source.WriteAssetsToZip(zw, names); err != nil {
		t.Fatalf("WriteAssetsToZip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(bundle.Bytes()), int64(bundle.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var bundled []string
	for _, entry := range zr.File {
		bundled = append(bundled, entry.Name)
	}
	if !slices.Equal(bundled, names) {
		t.Errorf("bundled assets = %v, want %v", bundled, names)
	}
	return names
}

func readConformanceBookCaches(t *testing.T, s *Shelf) []conformanceBookCache {
	t.Helper()

	entries, err := s.dbRoot.ReadDir(appFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []conformanceBookCache{}
		}
		t.Fatalf("ReadDir(%s): %v", appFolder, err)
	}

	observed := make([]conformanceBookCache, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, bookCacheFilePrefix) || !strings.HasSuffix(name, bookCacheFileSuffix) {
			continue
		}

		record := conformanceBookCache{Name: name, BookIDs: []string{}}
		if cache, err := s.readBookCacheFile(path.Join(appFolder, name)); err == nil {
			record.Usable = true
			for bookID := range cache.Books {
				record.BookIDs = append(record.BookIDs, bookID)
			}
			slices.Sort(record.BookIDs)
		}
		observed = append(observed, record)
	}

	slices.SortFunc(observed, func(a, b conformanceBookCache) int { return cmp.Compare(a.Name, b.Name) })
	return observed
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func orEmptyMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}

func formatConformanceTime(value util.JSONTime) string {
	if value.IsZero() {
		return ""
	}
	return time.Time(value).Format(time.RFC3339)
}

func formatConformanceDate(value util.JSONDate) string {
	if value.IsZero() {
		return ""
	}
	return time.Time(value).Format(time.DateOnly)
}

func assertConformanceEqual(t *testing.T, expected, observed conformanceReading) {
	t.Helper()

	want := marshalConformance(t, expected)
	got := marshalConformance(t, observed)
	if want == got {
		return
	}

	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := range max(len(wantLines), len(gotLines)) {
		wantLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		gotLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			t.Errorf("shelf reading differs from expected.json at line %d:\n  expected: %s\n  read:     %s",
				i+1, wantLine, gotLine)
			break
		}
	}
	t.Errorf("full reading:\nexpected:\n%s\nread:\n%s", want, got)
}

func marshalConformance(t *testing.T, reading conformanceReading) string {
	t.Helper()

	data, err := json.Marshal(reading, jsonopt.Disk())
	if err != nil {
		t.Fatalf("failed to marshal a conformance reading: %v", err)
	}
	return string(data)
}
