package shelf

import (
	"cmp"
	json "encoding/json/v2"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

/*
The Go half of the cross-language conformance suite. Its TypeScript twin is
frontend/src/api/pcloud/conformance.test.ts, and both read the same dataset from
testdata/conformance — see the README there for the contract.

The point is not to cover this package (the tests around it do that already) but
to pin the observable reading of a shelf to a file the pCloud client can be held
to as well. That client re-implements this walk against pCloud's listing API and
cannot import any of it, so a change here that is not mirrored there is
otherwise invisible until a phone shows the wrong library.
*/

// conformanceDatasetVersion is the expected.json shape this harness understands,
// matching schema_version in testdata/conformance/manifest.json. Both harnesses
// check it, so a dataset change that outpaces one of them fails loudly instead
// of being read as a behavior difference.
const conformanceDatasetVersion = 1

const conformanceRoot = "testdata/conformance"

type conformanceManifest struct {
	SchemaVersion int `json:"schema_version"`
	Cases         []struct {
		Name   string `json:"name"`
		Covers string `json:"covers"`
	} `json:"cases"`
}

// conformanceReading is what both implementations must report for one case.
type conformanceReading struct {
	Folders     []string               `json:"folders"`
	Books      []conformanceBook      `json:"books"`
	BookCaches []conformanceBookCache `json:"book_caches"`
}

type conformanceBook struct {
	Path                string              `json:"path"`
	Folders              []string            `json:"folders"`
	ID                  string              `json:"id"`
	Title               string              `json:"title"`
	Format              string              `json:"format"`
	Authors             []string            `json:"authors"`
	Tags                []string            `json:"tags"`
	Star                int                 `json:"star"`
	Cover               string              `json:"cover"`
	CoverPresent        bool                `json:"cover_present"`
	SchemaVersionOnDisk int                 `json:"schema_version_on_disk"`
	ReadOnly            bool                `json:"read_only"`
	CurrentSourceField  string              `json:"current_source_field"`
	CurrentSource       *string             `json:"current_source"`
	Sources             []conformanceSource `json:"sources"`
}

type conformanceSource struct {
	ID         string   `json:"id"`
	HasContent bool     `json:"has_content"`
	CharCount  int      `json:"char_count"`
	Assets     []string `json:"assets"`
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

// TestBookPackageConformanceDatasetIsComplete keeps the manifest and the case
// directories in step. Without it a case could be added and never run, which is
// the one failure mode a conformance suite cannot afford: it would report
// agreement it never checked.
func TestBookPackageConformanceDatasetIsComplete(t *testing.T) {
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

// decodeConformanceJSON refuses unknown fields, so a typo in the dataset is a
// failure rather than a silently unchecked expectation.
func decodeConformanceJSON(t *testing.T, filePath string, target any) {
	t.Helper()

	file, err := os.Open(filePath) //nolint:gosec // fixed test fixture path
	if err != nil {
		t.Fatalf("failed to open %s: %v", filePath, err)
	}
	defer file.Close() //nolint:errcheck // read-only fixture

	if err := json.UnmarshalRead(file, target, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("failed to decode %s: %v", filePath, err)
	}
}

// readConformanceCase opens one case's shelf and records what this build reads
// from it.
//
// The shelf is copied to a temp directory first: opening one creates a lock
// file and a scan cache under app/, and a fixture that a test run modifies
// would stop describing what was committed.
func readConformanceCase(t *testing.T, shelfDir string) conformanceReading {
	t.Helper()

	libRoot := filepath.Join(t.TempDir(), "shelf")
	if err := os.CopyFS(libRoot, os.DirFS(shelfDir)); err != nil {
		t.Fatalf("failed to copy the case shelf: %v", err)
	}

	// No BookCacheWriterID: the export would add a cache file of its own to
	// app/ and change what the book_caches expectation describes.
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	return conformanceReading{
		Folders:     s.collectExportFolders(),
		Books:      readConformanceBooks(t, s),
		BookCaches: readConformanceBookCaches(t, s),
	}
}

// readOnDiskSchemaVersion reads book.json's raw schema_version. The normalized
// BookMeta.SchemaVersion cannot report it: a pre-v1 book and a v1 book both read
// as 1 in memory, and that difference is exactly what the dataset records.
func readOnDiskSchemaVersion(t *testing.T, s *Shelf, bookPath string) int {
	t.Helper()

	file, err := s.dbRoot.Open(path.Join(bookPath, BookMetaFile))
	if err != nil {
		t.Fatalf("open book.json (%s): %v", bookPath, err)
	}
	defer file.Close() //nolint:errcheck // read-only probe

	var meta struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.UnmarshalRead(file, &meta, jsonopt.Read()); err != nil {
		t.Fatalf("decode book.json (%s): %v", bookPath, err)
	}
	return meta.SchemaVersion
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

		// The normalized SchemaVersion in meta cannot tell a pre-v1 book from a
		// v1 one, and that difference is exactly what the dataset records.
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
			Folders:              orEmpty(listing.Folders),
			ID:                  book.ID(),
			Title:               book.Title(),
			Format:              meta.Format,
			Authors:             orEmpty(meta.Authors),
			Tags:                orEmpty(meta.Tags),
			Star:                meta.Star,
			Cover:               meta.Cover,
			CoverPresent:        coverPresent,
			SchemaVersionOnDisk: onDiskSchema,
			ReadOnly:            errors.Is(book.EnsureWritable(), ErrUnsupportedBookSchemaVersion),
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
		// A book that never had a source has no sources/ folder at all.
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
			content.Close() //nolint:errcheck // read-only probe
		}

		observed = append(observed, conformanceSource{
			ID:         source.ID(),
			HasContent: hasContent,
			CharCount:  source.GetMeta().CharCount,
			Assets:     readConformanceAssets(t, s, source),
		})
	}
	return observed
}

// readConformanceAssets lists a source's illustrations and checks each one is
// reachable under the relative path the format documents. Nothing records the
// contents of assets/ — the filesystem is the list (see asset.go) — so the
// listing itself is what the pCloud client indexes from its own listing.
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
		asset.File.Close() //nolint:errcheck // read-only probe
	}

	slices.Sort(names)
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

// orEmpty replaces a nil slice with an empty one so the JSON comparison below
// sees [] on both sides: the TypeScript harness has no way to express nil.
func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
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
