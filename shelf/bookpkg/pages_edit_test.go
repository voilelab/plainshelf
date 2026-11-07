package bookpkg

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type assetSnapshot map[string]time.Time

func newEditableComic(t *testing.T, pageList string, assets ...string) (*Source, string) {
	t.Helper()

	book, _, libRoot := newTestBook(t, "comic-book", "Comic")
	source := newImagesSource(t, book, pageList, "")
	for _, name := range assets {
		if err := source.WriteAsset(name, []byte(name)); err != nil {
			t.Fatalf("WriteAsset(%s): %v", name, err)
		}
	}
	old := time.Now().Add(-time.Hour)
	for _, name := range assets {
		if err := os.Chtimes(filepath.Join(libRoot, source.FolderPath(), SourceAssetsFolder, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	return source, libRoot
}

func snapshotAssets(t *testing.T, libRoot string, source *Source) assetSnapshot {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(libRoot, source.FolderPath(), SourceAssetsFolder))
	if err != nil {
		t.Fatalf("ReadDir(assets): %v", err)
	}
	snapshot := assetSnapshot{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		snapshot[entry.Name()] = info.ModTime()
	}
	return snapshot
}

func currentPages(t *testing.T, source *Source) *PageList {
	t.Helper()

	list, err := source.Pages()
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	return list
}

func pageNames(list *PageList) []string {
	names := make([]string, len(list.Pages))
	for i, page := range list.Pages {
		names[i] = page.Name
	}
	return names
}

func readPageListFile(t *testing.T, source *Source) string {
	t.Helper()

	content, err := source.ReadContent()
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	return string(content)
}

func TestReplacePagesRenamesNoAsset(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\nb.jpg\nc.webp\n", "a.png", "b.jpg", "c.webp")
	before := snapshotAssets(t, libRoot, source)

	list, err := source.ReplacePages(currentPages(t, source).MD5Hash, []string{"c.webp", "a.png", "b.jpg"})
	if err != nil {
		t.Fatalf("ReplacePages: %v", err)
	}

	if got := readPageListFile(t, source); got != "c.webp\na.png\nb.jpg\n" {
		t.Errorf("source.txt = %q, want the new order", got)
	}
	if got := pageNames(list); !reflect.DeepEqual(got, []string{"c.webp", "a.png", "b.jpg"}) {
		t.Errorf("returned pages = %v, want the new order", got)
	}
	if list.MD5Hash != source.GetMeta().MD5Hash {
		t.Errorf("returned md5_hash = %q, want meta.json's %q", list.MD5Hash, source.GetMeta().MD5Hash)
	}
	if after := snapshotAssets(t, libRoot, source); !reflect.DeepEqual(after, before) {
		t.Errorf("assets after reorder = %v, want every file untouched: %v", after, before)
	}
}

func TestReplacePagesRejectsUnsafeNames(t *testing.T) {
	source, _ := newEditableComic(t, "a.png\n", "a.png")
	md5Hash := currentPages(t, source).MD5Hash

	for _, names := range [][]string{{"../a.png"}, {" a.png"}, {"a.png\nb.png"}, {"notes.txt"}} {
		if _, err := source.ReplacePages(md5Hash, names); !errors.Is(err, ErrInvalidPageList) {
			t.Errorf("ReplacePages(%q): err = %v, want ErrInvalidPageList", names, err)
		}
	}
	if got := readPageListFile(t, source); got != "a.png\n" {
		t.Errorf("source.txt = %q, want it untouched", got)
	}
}

func TestPageEditsRejectStaleMD5(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\nb.png\n", "a.png", "b.png")
	stale := currentPages(t, source).MD5Hash

	external := "b.png\na.png\n"
	if err := os.WriteFile(filepath.Join(libRoot, source.FolderPath(), SourceFile), []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotAssets(t, libRoot, source)

	if _, err := source.ReplacePages(stale, []string{"a.png"}); !errors.Is(err, ErrPageListConflict) {
		t.Errorf("ReplacePages: err = %v, want ErrPageListConflict", err)
	}
	if _, err := source.InsertPage(stale, 0, []byte("x"), ".png"); !errors.Is(err, ErrPageListConflict) {
		t.Errorf("InsertPage: err = %v, want ErrPageListConflict", err)
	}
	if _, err := source.DeletePage(stale, "a.png"); !errors.Is(err, ErrPageListConflict) {
		t.Errorf("DeletePage: err = %v, want ErrPageListConflict", err)
	}

	if got := readPageListFile(t, source); got != external {
		t.Errorf("source.txt = %q, want the external write kept", got)
	}
	if after := snapshotAssets(t, libRoot, source); !reflect.DeepEqual(after, before) {
		t.Errorf("assets = %v, want no write after a conflict: %v", after, before)
	}
}

func TestInsertPageAddsOneRandomAsset(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\nb.png\n", "a.png", "b.png")
	before := snapshotAssets(t, libRoot, source)

	list, err := source.InsertPage(currentPages(t, source).MD5Hash, 1, []byte("new"), ".PNG")
	if err != nil {
		t.Fatalf("InsertPage: %v", err)
	}

	names := pageNames(list)
	if len(names) != 3 || names[0] != "a.png" || names[2] != "b.png" {
		t.Fatalf("pages = %v, want the new page between a.png and b.png", names)
	}
	added := names[1]
	if !strings.HasSuffix(added, ".png") || added == "a.png" || added == "b.png" {
		t.Errorf("new page name = %q, want a fresh random name with a lower-case .png", added)
	}
	after := snapshotAssets(t, libRoot, source)
	if _, ok := after[added]; !ok || len(after) != len(before)+1 {
		t.Errorf("assets = %v, want exactly %q added", after, added)
	}
	for name, modTime := range before {
		if !after[name].Equal(modTime) {
			t.Errorf("asset %q was rewritten", name)
		}
	}
}

func TestInsertPageRejectsBadPosition(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\n", "a.png")
	md5Hash := currentPages(t, source).MD5Hash

	for _, position := range []int{-1, 2} {
		if _, err := source.InsertPage(md5Hash, position, []byte("x"), ".png"); !errors.Is(err, ErrInvalidPagePosition) {
			t.Errorf("InsertPage at %d: err = %v, want ErrInvalidPagePosition", position, err)
		}
	}
	if got := snapshotAssets(t, libRoot, source); len(got) != 1 {
		t.Errorf("assets = %v, want no page written", got)
	}
}

func TestDeletePageCases(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\ngone.png\nb.png\n", "a.png", "b.png", "stray.gif")

	list, err := source.DeletePage(currentPages(t, source).MD5Hash, "a.png")
	if err != nil {
		t.Fatalf("DeletePage(listed): %v", err)
	}
	if got := readPageListFile(t, source); got != "gone.png\nb.png\n" {
		t.Errorf("source.txt = %q after deleting a.png", got)
	}
	if _, ok := snapshotAssets(t, libRoot, source)["a.png"]; ok {
		t.Error("a.png still on disk after deleting the page")
	}

	list, err = source.DeletePage(list.MD5Hash, "gone.png")
	if err != nil {
		t.Fatalf("DeletePage(missing placeholder): %v", err)
	}
	if got := pageNames(list); !reflect.DeepEqual(got, []string{"b.png"}) {
		t.Errorf("pages = %v after removing the missing page, want [b.png]", got)
	}

	list, err = source.DeletePage(list.MD5Hash, "stray.gif")
	if err != nil {
		t.Fatalf("DeletePage(unlisted): %v", err)
	}
	if len(list.Unlisted) != 0 || readPageListFile(t, source) != "b.png\n" {
		t.Errorf("after deleting the unlisted image: unlisted = %v, source.txt = %q", list.Unlisted, readPageListFile(t, source))
	}

	if _, err := source.DeletePage(list.MD5Hash, "nowhere.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("DeletePage(unknown): err = %v, want ErrAssetNotFound", err)
	}
	if got := snapshotAssets(t, libRoot, source); !reflect.DeepEqual(keys(got), []string{"b.png"}) {
		t.Errorf("assets = %v, want only b.png left", got)
	}
}

func TestPageEditsNeedImagesSource(t *testing.T) {
	book, _, _ := newTestBook(t, "prose-book", "Prose")
	source, err := book.NewSource(strings.NewReader("prose"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	if _, err := source.ReplacePages(source.GetMeta().MD5Hash, nil); !errors.Is(err, ErrNotImageSource) {
		t.Errorf("ReplacePages on text: err = %v, want ErrNotImageSource", err)
	}
	if err := source.SetPageDirection(PageDirectionRTL); !errors.Is(err, ErrInvalidPageDirection) {
		t.Errorf("SetPageDirection(rtl) on text: err = %v, want ErrInvalidPageDirection", err)
	}
}

func TestSetPageDirectionOnlyRewritesMeta(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\n", "a.png")
	listPath := filepath.Join(libRoot, source.FolderPath(), SourceFile)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(listPath, old, old); err != nil {
		t.Fatal(err)
	}

	if err := source.SetPageDirection(PageDirectionRTL); err != nil {
		t.Fatalf("SetPageDirection(rtl): %v", err)
	}
	if raw := readSourceMetaKeys(t, libRoot, source); !strings.Contains(raw, `"page_direction": "rtl"`) {
		t.Errorf("meta.json = %s, want page_direction rtl", raw)
	}
	if err := source.SetPageDirection(PageDirectionLTR); err != nil {
		t.Fatalf("SetPageDirection(ltr): %v", err)
	}
	if raw := readSourceMetaKeys(t, libRoot, source); strings.Contains(raw, "page_direction") {
		t.Errorf("meta.json = %s, want page_direction dropped for ltr", raw)
	}
	if err := source.SetPageDirection("ttb"); !errors.Is(err, ErrInvalidPageDirection) {
		t.Errorf("SetPageDirection(ttb): err = %v, want ErrInvalidPageDirection", err)
	}

	info, err := os.Stat(listPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Error("source.txt was rewritten by a page direction change")
	}
}

func keys(snapshot assetSnapshot) []string {
	names := make([]string, 0, len(snapshot))
	for name := range snapshot {
		names = append(names, name)
	}
	return names
}

func TestInsertPageKeepsImageListedOnFail(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\n", "a.png")
	md5Hash := currentPages(t, source).MD5Hash
	metaPath := filepath.Join(libRoot, source.FolderPath(), SourceMetaFile)
	if err := os.Remove(metaPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(metaPath, "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := source.InsertPage(md5Hash, 1, []byte("new"), ".png"); err == nil {
		t.Fatal("InsertPage succeeded with meta.json unwritable")
	}

	names := parsePageNames([]byte(readPageListFile(t, source)))
	if len(names) != 2 {
		t.Fatalf("source.txt names = %v, want the new page written before meta.json failed", names)
	}
	if _, ok := snapshotAssets(t, libRoot, source)[names[1]]; !ok {
		t.Errorf("image %q was deleted although source.txt lists it", names[1])
	}
}

func TestInsertPageCleansUpUnlistedOnFail(t *testing.T) {
	source, libRoot := newEditableComic(t, "a.png\n", "a.png")
	md5Hash := currentPages(t, source).MD5Hash
	listPath := filepath.Join(libRoot, source.FolderPath(), SourceFile)
	if err := os.Chmod(filepath.Dir(listPath), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(listPath), 0o755) })
	if f, err := os.CreateTemp(filepath.Dir(listPath), "probe"); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced for this user")
	}

	if _, err := source.InsertPage(md5Hash, 1, []byte("new"), ".png"); err == nil {
		t.Fatal("InsertPage succeeded with source.txt unwritable")
	}

	if got := readPageListFile(t, source); got != "a.png\n" {
		t.Errorf("source.txt = %q, want it untouched", got)
	}
	if got := keys(snapshotAssets(t, libRoot, source)); !reflect.DeepEqual(got, []string{"a.png"}) {
		t.Errorf("assets = %v, want the unlisted new image removed", got)
	}
}
