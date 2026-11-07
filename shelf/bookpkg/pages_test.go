package bookpkg

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newImagesSource(t *testing.T, book *Book, pageList, direction string) *Source {
	t.Helper()

	source, err := book.NewSourceWithOptions(strings.NewReader(pageList),
		NewSourceOptions{Format: BookFormatImages, PageDirection: direction})
	if err != nil {
		t.Fatalf("NewSourceWithOptions(images): %v", err)
	}
	return source
}

func readSourceMetaKeys(t *testing.T, libRoot string, source *Source) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(libRoot, source.FolderPath(), SourceMetaFile))
	if err != nil {
		t.Fatalf("ReadFile(meta.json): %v", err)
	}
	return string(raw)
}

func TestImagesSourceMirrorsFormatToBook(t *testing.T) {
	book, _, _ := newTestBook(t, "comic-book", "Comic")
	source := newImagesSource(t, book, "p-a.png\n", "")

	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}
	if got := book.GetMeta().Format; got != BookFormatImages {
		t.Fatalf("book.json format = %q, want images", got)
	}

	meta := book.GetMeta()
	meta.Title = "Renamed"
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta on an images book: %v", err)
	}
}

func TestImagesSourceStatsSkipCharCount(t *testing.T) {
	book, _, _ := newTestBook(t, "comic-book", "Comic")
	source := newImagesSource(t, book, "p-a.png\np-b.jpg\n", "")

	meta := source.GetMeta()
	if meta.CharCount != 0 {
		t.Errorf("char_count = %d, want 0 for an images source", meta.CharCount)
	}
	if meta.LineCount != 2 || meta.MD5Hash == "" {
		t.Errorf("line_count = %d, md5_hash = %q; want 2 and a hash of source.txt", meta.LineCount, meta.MD5Hash)
	}
	if source.HoldsText() {
		t.Error("HoldsText() = true for an images source")
	}

	if err := source.UpdateContent(strings.NewReader("p-b.jpg\n")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}
	if got := source.GetMeta(); got.CharCount != 0 || got.LineCount != 1 {
		t.Errorf("after update char_count = %d, line_count = %d; want 0 and 1", got.CharCount, got.LineCount)
	}
}

func TestPageDirectionWrittenOnlyForRTL(t *testing.T) {
	book, _, libRoot := newTestBook(t, "comic-book", "Comic")

	rtl := newImagesSource(t, book, "", PageDirectionRTL)
	if got := rtl.GetMeta().PageDirection; got != PageDirectionRTL {
		t.Errorf("page_direction = %q, want rtl", got)
	}
	if raw := readSourceMetaKeys(t, libRoot, rtl); !strings.Contains(raw, `"page_direction": "rtl"`) {
		t.Errorf("meta.json = %s, want page_direction rtl", raw)
	}

	ltr := newImagesSource(t, book, "", PageDirectionLTR)
	text, err := book.NewSource(strings.NewReader("prose"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	for _, source := range []*Source{ltr, text} {
		if raw := readSourceMetaKeys(t, libRoot, source); strings.Contains(raw, "page_direction") {
			t.Errorf("meta.json = %s, want no page_direction key for the default", raw)
		}
	}
}

func TestPageDirectionRejectsBadValues(t *testing.T) {
	book, _, _ := newTestBook(t, "comic-book", "Comic")

	cases := []NewSourceOptions{
		{Format: BookFormatText, PageDirection: PageDirectionRTL},
		{Format: BookFormatMarkdown, PageDirection: PageDirectionRTL},
		{Format: BookFormatImages, PageDirection: "ttb"},
	}
	for _, options := range cases {
		_, err := book.NewSourceWithOptions(strings.NewReader(""), options)
		if !errors.Is(err, ErrInvalidPageDirection) {
			t.Errorf("%+v: err = %v, want ErrInvalidPageDirection", options, err)
		}
	}
}

func TestPageListRejectsNonAssetNames(t *testing.T) {
	book, _, _ := newTestBook(t, "comic-book", "Comic")
	source := newImagesSource(t, book, "p-a.png\n", "")

	for _, list := range []string{
		"../escape.png\n",
		"assets/p-a.png\n",
		"notes.txt\n",
		".hidden.png\n",
		"p-a.png\nhttps://example.com/x.png\n",
	} {
		_, err := book.NewSourceWithOptions(strings.NewReader(list), NewSourceOptions{Format: BookFormatImages})
		if !errors.Is(err, ErrInvalidPageList) {
			t.Errorf("create with %q: err = %v, want ErrInvalidPageList", list, err)
		}
		if err := source.UpdateContent(strings.NewReader(list)); !errors.Is(err, ErrInvalidPageList) {
			t.Errorf("update with %q: err = %v, want ErrInvalidPageList", list, err)
		}
	}

	content, err := source.ReadContent()
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	if string(content) != "p-a.png\n" {
		t.Errorf("source.txt = %q after rejected updates, want it untouched", content)
	}

	tooLarge := strings.Repeat("p-a.png\n", maxPageListBytes/8+1)
	if err := source.UpdateContent(strings.NewReader(tooLarge)); !errors.Is(err, ErrInvalidPageList) {
		t.Errorf("oversized list: err = %v, want ErrInvalidPageList", err)
	}
}

func TestTextSourceAcceptsAnyContent(t *testing.T) {
	book, _, _ := newTestBook(t, "prose-book", "Prose")
	source, err := book.NewSource(strings.NewReader("../escape.png\n"))
	if err != nil {
		t.Fatalf("NewSource with path-like text: %v", err)
	}
	if err := source.UpdateContent(strings.NewReader("notes.txt\n")); err != nil {
		t.Fatalf("UpdateContent with path-like text: %v", err)
	}
	if _, err := source.Pages(); !errors.Is(err, ErrNotImageSource) {
		t.Errorf("Pages() on a txt source: err = %v, want ErrNotImageSource", err)
	}
}

func TestPagesReportsMissingAndUnlisted(t *testing.T) {
	book, _, libRoot := newTestBook(t, "comic-book", "Comic")
	list := "\xef\xbb\xbfp-c.webp\r\n\r\n  p-a.png  \r\np-gone.jpg\np-a.png\n"
	source := newImagesSource(t, book, list, "")

	for _, name := range []string{"p-a.png", "p-c.webp", "p-extra.gif", "readme.txt"} {
		writeAsset(t, libRoot, source, name, []byte(name))
	}

	got, err := source.Pages()
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	want := &PageList{
		Pages: []Page{
			{Name: "p-c.webp"},
			{Name: "p-a.png"},
			{Name: "p-gone.jpg", Missing: true},
			{Name: "p-a.png"},
		},
		Unlisted: []string{"p-extra.gif"},
		MD5Hash:  source.GetMeta().MD5Hash,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Pages() = %+v, want %+v", got, want)
	}
}

func TestPagesToleratesHandEditedList(t *testing.T) {
	book, _, libRoot := newTestBook(t, "comic-book", "Comic")
	source := newImagesSource(t, book, "p-a.png\n", "")

	sourceFile := filepath.Join(libRoot, source.FolderPath(), SourceFile)
	if err := os.WriteFile(sourceFile, []byte("../escape.png\np-a.png\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeAsset(t, libRoot, source, "p-a.png", []byte("a"))

	got, err := source.Pages()
	if err != nil {
		t.Fatalf("Pages on a hand-edited list: %v", err)
	}
	want := []Page{{Name: "../escape.png", Missing: true}, {Name: "p-a.png"}}
	if !reflect.DeepEqual(got.Pages, want) {
		t.Errorf("pages = %+v, want %+v", got.Pages, want)
	}

	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("Remove(source.txt): %v", err)
	}
	got, err = source.Pages()
	if err != nil {
		t.Fatalf("Pages without source.txt: %v", err)
	}
	if len(got.Pages) != 0 || !reflect.DeepEqual(got.Unlisted, []string{"p-a.png"}) {
		t.Errorf("Pages without source.txt = %+v, want no pages and p-a.png unlisted", got)
	}
}
