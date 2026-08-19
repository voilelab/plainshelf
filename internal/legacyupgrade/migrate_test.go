package legacyupgrade

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/shelf"
)

// The tests build a shelf through its public API, close it, edit the files on
// disk the way an older build would have written them, and only then run the
// migration against a freshly opened shelf. Going through a new shelf each time
// keeps the book cache out of the assertions and matches how the tool is
// actually used: a separate process opening a shelf nobody else has open.

func withShelf(t *testing.T, libRoot string, use func(sh *shelf.Shelf)) {
	t.Helper()

	sh, err := shelf.NewShelf(&shelf.ShelfConf{
		LibRoot:      libRoot,
		LockMode:     "none",
		ScanInterval: "0s",
	})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer sh.Close()

	if err := sh.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	use(sh)
}

func migrate(t *testing.T, libRoot string, opts Options) *Report {
	t.Helper()

	var report *Report
	withShelf(t, libRoot, func(sh *shelf.Shelf) {
		var err error
		report, err = MigrateShelf(sh, opts)
		if err != nil {
			t.Fatalf("MigrateShelf: %v", err)
		}
	})
	return report
}

// legacySource is where one prepared source lives on disk.
type legacySource struct {
	bookID    string
	sourceID  string
	bookDir   string // absolute
	sourceDir string // absolute
}

func (s legacySource) contentPath() string { return filepath.Join(s.sourceDir, shelf.SourceFile) }
func (s legacySource) metaPath() string    { return filepath.Join(s.sourceDir, shelf.SourceMetaFile) }
func (s legacySource) bookMetaPath() string {
	return filepath.Join(s.bookDir, shelf.BookMetaFile)
}

func (s legacySource) content(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(s.contentPath())
	if err != nil {
		t.Fatalf("read source.txt: %v", err)
	}
	return string(content)
}

func (s legacySource) meta(t *testing.T) map[string]any {
	t.Helper()
	return readJSONFile(t, s.metaPath())
}

func (s legacySource) bookMeta(t *testing.T) map[string]any {
	t.Helper()
	return readJSONFile(t, s.bookMetaPath())
}

func readJSONFile(t *testing.T, filePath string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode %s: %v", filePath, err)
	}
	return decoded
}

// newLegacyShelf creates a book with one source holding content, then rewrites
// its meta.json the way a build without source-level format metadata would
// have: no schema_version, no format, chapters carried by splitJSON.
func newLegacyShelf(t *testing.T, content, splitJSON string) (string, legacySource) {
	t.Helper()

	libRoot := t.TempDir()
	var prepared legacySource

	withShelf(t, libRoot, func(sh *shelf.Shelf) {
		book, err := sh.NewBook(nil, "Legacy book")
		if err != nil {
			t.Fatalf("NewBook: %v", err)
		}
		source, err := book.NewSource(strings.NewReader(content))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			t.Fatalf("SetCurrentSource: %v", err)
		}
		prepared = legacySource{
			bookID:    book.ID(),
			sourceID:  source.ID(),
			bookDir:   filepath.Join(libRoot, filepath.FromSlash(book.FolderPath())),
			sourceDir: filepath.Join(libRoot, filepath.FromSlash(source.FolderPath())),
		}
	})

	writeLegacyMeta(t, prepared, splitJSON)
	return libRoot, prepared
}

func writeLegacyMeta(t *testing.T, source legacySource, splitJSON string) {
	t.Helper()

	legacy := `{"id":"` + source.sourceID + `","created_at":"2026-01-01T00:00:00Z",` +
		`"comment":"legacy","split_config":` + splitJSON + `}`
	if err := os.WriteFile(source.metaPath(), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy meta: %v", err)
	}
}

// setBookFormat rewrites only book.json's compatibility mirror.
func setBookFormat(t *testing.T, source legacySource, format string) {
	t.Helper()

	meta := source.bookMeta(t)
	meta["format"] = format
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("encode book.json: %v", err)
	}
	if err := os.WriteFile(source.bookMetaPath(), raw, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}
}

func onlyResult(t *testing.T, report *Report) SourceResult {
	t.Helper()
	if len(report.Results) != 1 {
		t.Fatalf("got %d results %+v, want 1", len(report.Results), report.Results)
	}
	return report.Results[0]
}

func TestMigrateShelfRewritesLegacySplits(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		splitJSON   string
		wantContent string
		wantChapter int
	}{
		{
			name: "line count", content: "a\nb\nc\nd",
			splitJSON:   `{"type":"line_count","line_count":2}`,
			wantContent: "## Part 1\n\na\nb\n## Part 2\n\nc\nd", wantChapter: 2,
		},
		{
			name: "boundary", content: "a\nb\nc",
			splitJSON:   `{"type":"boundary","boundaries":[1,3]}`,
			wantContent: "## Part 1\n\na\nb\n## Part 2\n\nc", wantChapter: 2,
		},
		{
			name: "regex", content: "Chapter 1\nText\nChapter 2",
			splitJSON:   `{"type":"regex","regex":"^Chapter \\d+$"}`,
			wantContent: "## Chapter 1\nText\n## Chapter 2", wantChapter: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			libRoot, source := newLegacyShelf(t, test.content, test.splitJSON)

			result := onlyResult(t, migrate(t, libRoot, Options{}))
			if result.Action != ActionRewrote {
				t.Fatalf("action = %q, want %q (%v)", result.Action, ActionRewrote, result.Err)
			}
			if result.Format != shelf.BookFormatMarkdown {
				t.Errorf("format = %q, want md", result.Format)
			}
			if result.Chapters != test.wantChapter {
				t.Errorf("chapters = %d, want %d", result.Chapters, test.wantChapter)
			}
			if got := source.content(t); got != test.wantContent {
				t.Errorf("source.txt = %q, want %q", got, test.wantContent)
			}

			meta := source.meta(t)
			if meta["schema_version"] != float64(shelf.SourceMetaSchemaVersion) {
				t.Errorf("schema_version = %v, want 1", meta["schema_version"])
			}
			if meta["format"] != shelf.BookFormatMarkdown {
				t.Errorf("format = %v, want md", meta["format"])
			}
			if split, _ := meta["split_config"].(map[string]any); split["type"] != "" {
				t.Errorf("split_config = %v, want the ignored empty type", split)
			}
		})
	}
}

func TestMigrateShelfStampsSourcesThatNeedNoRewrite(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		splitJSON    string
		bookFormat   string
		wantFormat   string
		wantChapters int
		wantReason   bool
	}{
		{
			name: "plain text with no split", content: "a\nb",
			splitJSON: `{"type":""}`, bookFormat: shelf.BookFormatText,
			wantFormat: shelf.BookFormatText, wantChapters: 1,
		},
		{
			name: "markdown with no split", content: "## One\na\n## Two\nb",
			splitJSON: `{"type":""}`, bookFormat: shelf.BookFormatMarkdown,
			wantFormat: shelf.BookFormatMarkdown, wantChapters: 2,
		},
		{
			// The user asked for chapters and the text already spells them out,
			// so the source ends up Markdown even though its bytes do not move.
			name: "plain text whose content already has headings", content: "## One\na\n## Two\nb",
			splitJSON: `{"type":"line_count","line_count":1}`, bookFormat: shelf.BookFormatText,
			wantFormat: shelf.BookFormatMarkdown, wantChapters: 2, wantReason: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			libRoot, source := newLegacyShelf(t, test.content, test.splitJSON)
			setBookFormat(t, source, test.bookFormat)

			before, err := os.Stat(source.contentPath())
			if err != nil {
				t.Fatalf("stat source.txt: %v", err)
			}

			result := onlyResult(t, migrate(t, libRoot, Options{}))
			if result.Action != ActionStamped {
				t.Fatalf("action = %q, want %q (%v)", result.Action, ActionStamped, result.Err)
			}
			if result.Format != test.wantFormat {
				t.Errorf("format = %q, want %q", result.Format, test.wantFormat)
			}
			if result.Chapters != test.wantChapters {
				t.Errorf("chapters = %d, want %d", result.Chapters, test.wantChapters)
			}
			if (result.Reason != "") != test.wantReason {
				t.Errorf("reason = %q, want a reason: %v", result.Reason, test.wantReason)
			}

			if got := source.content(t); got != test.content {
				t.Errorf("source.txt = %q, want it unchanged", got)
			}
			after, err := os.Stat(source.contentPath())
			if err != nil {
				t.Fatalf("stat source.txt: %v", err)
			}
			if !after.ModTime().Equal(before.ModTime()) {
				t.Errorf("source.txt was rewritten")
			}
			if got := source.meta(t)["format"]; got != test.wantFormat {
				t.Errorf("persisted format = %v, want %q", got, test.wantFormat)
			}
		})
	}
}

// TestMigrateShelfAppliesTheShelfWideDefaultSplit covers the fallback the
// reader performs for a legacy source with no split configuration of its own.
func TestMigrateShelfAppliesTheShelfWideDefaultSplit(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb\nc\nd", `{"type":""}`)

	report := migrate(t, libRoot, Options{
		DefaultSplit: shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 2},
	})

	result := onlyResult(t, report)
	if result.Action != ActionRewrote {
		t.Fatalf("action = %q, want %q (%v)", result.Action, ActionRewrote, result.Err)
	}
	if got, want := source.content(t), "## Part 1\n\na\nb\n## Part 2\n\nc\nd"; got != want {
		t.Errorf("source.txt = %q, want %q", got, want)
	}
}

// TestMigrateShelfStampsARegexThatMatchesNothing covers the case a real shelf
// turned up: a pattern that compiles and finds no line is splitting nothing, so
// the source already reads as one section and only needs its metadata. Its bytes
// and its format stay as they are, which is what the reader has been showing.
func TestMigrateShelfStampsARegexThatMatchesNothing(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb", `{"type":"regex","regex":"^ZZZ$"}`)
	before := source.content(t)

	result := onlyResult(t, migrate(t, libRoot, Options{}))

	if result.Action != ActionStamped {
		t.Fatalf("action = %q (%v), want %q", result.Action, result.Err, ActionStamped)
	}
	if result.Format != shelf.BookFormatText {
		t.Errorf("format = %q, want the format it already rendered as", result.Format)
	}
	if result.Reason == "" {
		t.Errorf("want a reason explaining why the split produced nothing")
	}
	if got := source.content(t); got != before {
		t.Errorf("source.txt = %q, want it untouched", got)
	}
	if _, ok := source.meta(t)["schema_version"]; !ok {
		t.Errorf("meta.json was not stamped: %v", source.meta(t))
	}
}

func TestMigrateShelfLeavesSourcesItCannotReproduce(t *testing.T) {
	tests := []struct {
		name      string
		splitJSON string
		wantErr   error
	}{
		{"regex RE2 rejects", `{"type":"regex","regex":"^(?=x)x$"}`, ErrUnsupportedSplitRegex},
		{"split type this build does not know", `{"type":"chapters"}`, ErrUnrecognizedSplitType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			libRoot, source := newLegacyShelf(t, "a\nb", test.splitJSON)
			before := source.content(t)

			result := onlyResult(t, migrate(t, libRoot, Options{}))
			if result.Action != ActionNeedsAttention {
				t.Fatalf("action = %q, want %q", result.Action, ActionNeedsAttention)
			}
			if !errors.Is(result.Err, test.wantErr) {
				t.Errorf("error = %v, want %v", result.Err, test.wantErr)
			}
			if result.Reason == "" {
				t.Errorf("a skipped source must carry a reason an operator can act on")
			}
			if got := source.content(t); got != before {
				t.Errorf("source.txt = %q, want it untouched", got)
			}

			meta := source.meta(t)
			if _, ok := meta["schema_version"]; ok {
				t.Errorf("meta.json gained schema_version: %v", meta)
			}
			if _, ok := meta["format"]; ok {
				t.Errorf("meta.json gained format: %v", meta)
			}
		})
	}
}

func TestMigrateShelfSkipsSchemaV1Sources(t *testing.T) {
	libRoot := t.TempDir()
	withShelf(t, libRoot, func(sh *shelf.Shelf) {
		book, err := sh.NewBook(nil, "Modern")
		if err != nil {
			t.Fatalf("NewBook: %v", err)
		}
		if _, err := book.NewSourceWithOptions(strings.NewReader("## One\nbody"),
			shelf.NewSourceOptions{Format: shelf.BookFormatMarkdown}); err != nil {
			t.Fatalf("NewSourceWithOptions: %v", err)
		}
	})

	result := onlyResult(t, migrate(t, libRoot, Options{}))
	if result.Action != ActionAlreadyV1 {
		t.Fatalf("action = %q, want %q", result.Action, ActionAlreadyV1)
	}
}

func TestMigrateShelfRefusesASourceWrittenByANewerBuild(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb", `{"type":""}`)
	future := `{"schema_version":99,"id":"` + source.sourceID +
		`","created_at":"2026-01-01T00:00:00Z","comment":"","split_config":{"type":""}}`
	if err := os.WriteFile(source.metaPath(), []byte(future), 0o644); err != nil {
		t.Fatalf("write future meta: %v", err)
	}

	// A source claiming a newer schema is not legacy, so it is skipped rather
	// than failed - and either way nothing is written to it.
	result := onlyResult(t, migrate(t, libRoot, Options{}))
	if result.Action != ActionAlreadyV1 {
		t.Fatalf("action = %q, want %q", result.Action, ActionAlreadyV1)
	}
	if got := source.meta(t)["schema_version"]; got != float64(99) {
		t.Errorf("schema_version = %v, want it untouched", got)
	}
}

// snapshotTree records every file under root with its bytes and mod time.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[path] = info.ModTime().Format(time.RFC3339Nano) + "\x00" + string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return tree
}

func TestMigrateShelfDryRunWritesNothing(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb\nc\nd", `{"type":"line_count","line_count":2}`)
	booksRoot := filepath.Dir(source.bookDir)

	before := snapshotTree(t, booksRoot)
	report := migrate(t, libRoot, Options{DryRun: true})
	after := snapshotTree(t, booksRoot)

	result := onlyResult(t, report)
	if result.Action != ActionRewrote {
		t.Fatalf("action = %q, want the dry run to still report the full plan", result.Action)
	}
	if result.Chapters != 2 {
		t.Errorf("chapters = %d, want the dry run to compute them", result.Chapters)
	}

	if len(before) != len(after) {
		t.Fatalf("file count changed from %d to %d", len(before), len(after))
	}
	for path, state := range before {
		if after[path] != state {
			t.Errorf("%s changed during a dry run", path)
		}
	}
}

// TestMigrateShelfTakesTheFormatFromTheBookMirror pins the format resolution a
// legacy source gets: book.json's mirror, which is all that is left to consult.
func TestMigrateShelfTakesTheFormatFromTheBookMirror(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "plain text", `{"type":""}`)
	setBookFormat(t, source, shelf.BookFormatMarkdown)

	result := onlyResult(t, migrate(t, libRoot, Options{}))
	if result.Format != shelf.BookFormatMarkdown {
		t.Errorf("format = %q, want the mirror's md", result.Format)
	}
}

func TestMigrateShelfFallsBackToPlainTextWithoutAnyFormat(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "plain text", `{"type":""}`)
	setBookFormat(t, source, "")

	result := onlyResult(t, migrate(t, libRoot, Options{}))
	if result.Format != shelf.BookFormatText {
		t.Errorf("format = %q, want txt", result.Format)
	}
}

func TestMigrateShelfRefreshesTheBookFormatMirror(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb\nc\nd", `{"type":"line_count","line_count":2}`)
	setBookFormat(t, source, shelf.BookFormatText)

	migrate(t, libRoot, Options{})

	if got := source.bookMeta(t)["format"]; got != shelf.BookFormatMarkdown {
		t.Errorf("book.json format = %v, want md after the current source became Markdown", got)
	}
}

func TestMigrateShelfIsIdempotent(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb\nc\nd", `{"type":"line_count","line_count":2}`)

	migrate(t, libRoot, Options{})
	migrated := source.content(t)
	booksRoot := filepath.Dir(source.bookDir)
	before := snapshotTree(t, booksRoot)

	second := onlyResult(t, migrate(t, libRoot, Options{}))
	if second.Action != ActionAlreadyV1 {
		t.Fatalf("second run action = %q, want %q", second.Action, ActionAlreadyV1)
	}
	if got := source.content(t); got != migrated {
		t.Errorf("source.txt = %q, want %q", got, migrated)
	}
	for path, state := range snapshotTree(t, booksRoot) {
		if before[path] != state {
			t.Errorf("%s changed on a second run", path)
		}
	}
}

// TestMigrateShelfRecoversFromAnInterruptedRewrite covers the crash window the
// write order is chosen for: the text was replaced but meta.json never was.
func TestMigrateShelfRecoversFromAnInterruptedRewrite(t *testing.T) {
	libRoot, source := newLegacyShelf(t, "a\nb\nc\nd", `{"type":"line_count","line_count":2}`)
	halfDone := "## Part 1\n\na\nb\n## Part 2\n\nc\nd"
	if err := os.WriteFile(source.contentPath(), []byte(halfDone), 0o644); err != nil {
		t.Fatalf("write interrupted content: %v", err)
	}

	result := onlyResult(t, migrate(t, libRoot, Options{}))
	if result.Action != ActionStamped {
		t.Fatalf("action = %q, want %q", result.Action, ActionStamped)
	}
	if got := source.content(t); got != halfDone {
		t.Errorf("source.txt = %q, want the headings not to be inserted twice", got)
	}
	if got := source.meta(t)["format"]; got != shelf.BookFormatMarkdown {
		t.Errorf("format = %v, want md", got)
	}
}

func TestMigrateShelfBookScoping(t *testing.T) {
	libRoot, first := newLegacyShelf(t, "a\nb\nc\nd", `{"type":"line_count","line_count":2}`)

	var second legacySource
	withShelf(t, libRoot, func(sh *shelf.Shelf) {
		book, err := sh.NewBook(nil, "Untouched")
		if err != nil {
			t.Fatalf("NewBook: %v", err)
		}
		source, err := book.NewSource(strings.NewReader("x\ny\nz"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			t.Fatalf("SetCurrentSource: %v", err)
		}
		second = legacySource{
			bookID:    book.ID(),
			sourceID:  source.ID(),
			bookDir:   filepath.Join(libRoot, filepath.FromSlash(book.FolderPath())),
			sourceDir: filepath.Join(libRoot, filepath.FromSlash(source.FolderPath())),
		}
	})
	writeLegacyMeta(t, second, `{"type":"line_count","line_count":1}`)

	report := migrate(t, libRoot, Options{BookIDs: []string{first.bookID}})
	if report.BooksScanned != 1 {
		t.Errorf("BooksScanned = %d, want 1", report.BooksScanned)
	}
	result := onlyResult(t, report)
	if result.BookID != first.bookID {
		t.Errorf("migrated book %s, want %s", result.BookID, first.bookID)
	}
	if got := second.content(t); got != "x\ny\nz" {
		t.Errorf("out-of-scope source.txt = %q, want it untouched", got)
	}
}

func TestMigrateShelfRejectsAnUnknownBookID(t *testing.T) {
	libRoot, _ := newLegacyShelf(t, "a\nb", `{"type":""}`)

	withShelf(t, libRoot, func(sh *shelf.Shelf) {
		_, err := MigrateShelf(sh, Options{BookIDs: []string{"no-such-book"}})
		if err == nil {
			t.Fatalf("MigrateShelf accepted an unknown book ID")
		}
		if !strings.Contains(err.Error(), "no-such-book") {
			t.Errorf("error %q does not name the missing book", err)
		}
	})
}
