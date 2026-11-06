package server

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/gif"
	"image/jpeg"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
)

func TestCBZImportPages(t *testing.T) {
	app := newTestApp(t)
	data, _ := app.ShelfManager().GetShelf("default_shelf")
	png := testutil.OnePixelPNG()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var jpg, gifData bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifData, img, nil); err != nil {
		t.Fatal(err)
	}
	webp, err := base64.StdEncoding.DecodeString("UklGRjwAAABXRUJQVlA4IDAAAADQAQCdASoBAAEAAUAmJaACdLoB+AADsAD+8ut//NgVzXPv9//S4P0uD9Lg/9KQAAA=")
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string][]byte{
		"chapter10/1.png": png,
		"chapter2/10.jpg": jpg.Bytes(),
		"chapter2/2.PNG":  png,
		"chapter2/1.webp": webp,
		"chapter2/11.gif": gifData.Bytes(),
		"ComicInfo.xml":   []byte(`<ComicInfo><Title>Comic title</Title><Writer>A, B</Writer><Manga>YesAndRightToLeft</Manga></ComicInfo>`),
		"Thumbs.db":       []byte("ignored"), "__MACOSX/1.jpg": []byte("ignored"), ".hidden/1.jpg": []byte("ignored"), "chapter2/.cover.jpg": []byte("ignored"),
	}
	archive := testutil.BuildCBZ(t, entries)
	book, err := app.handlers.imports.importCBZ(data, bytes.NewReader(archive), int64(len(archive)), "comic.cbz", "comic", nil)
	if err != nil {
		t.Fatal(err)
	}
	meta := book.GetMeta()
	if meta.Title != "Comic title" || meta.Format != "images" || strings.Join(meta.Authors, ",") != "A,B" {
		t.Fatalf("meta = %+v", meta)
	}
	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		t.Fatal(err)
	}
	if source.GetMeta().PageDirection != "rtl" {
		t.Fatal("missing RTL direction")
	}
	list, err := source.Pages()
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Pages) != 5 || len(list.Unlisted) != 0 {
		t.Fatalf("pages = %+v", list)
	}
	for i, original := range []string{"chapter2/1.webp", "chapter2/2.PNG", "chapter2/10.jpg", "chapter2/11.gif", "chapter10/1.png"} {
		ext := filepath.Ext(original)
		page := list.Pages[i]
		if page.Missing || filepath.Ext(page.Name) != ext || len(strings.TrimSuffix(page.Name, ext)) < 20 {
			t.Fatalf("page %d = %+v", i, page)
		}
		asset, err := source.OpenAsset(page.Name)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := io.ReadAll(asset.File)
		asset.File.Close()
		if err != nil || !bytes.Equal(stored, entries[original]) {
			t.Fatalf("asset bytes changed: %v", err)
		}
	}
	cover, err := os.ReadFile(filepath.Join(app.conf.Shelves[0].LibRoot, book.PackagePath(), meta.Cover))
	if err != nil || !bytes.Equal(cover, webp) || filepath.Ext(meta.Cover) != ".webp" {
		t.Fatalf("cover changed: %v", err)
	}
	err = filepath.WalkDir(app.conf.Shelves[0].LibRoot, func(path string, entry fs.DirEntry, err error) error {
		if strings.HasSuffix(strings.ToLower(path), ".cbz") {
			t.Errorf("original archive stored: %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCBZFailuresLeaveNoBook(t *testing.T) {
	png := testutil.OnePixelPNG()
	huge := bytes.Clone(png)
	binary.BigEndian.PutUint32(huge[16:20], 10001)
	binary.BigEndian.PutUint32(huge[20:24], 10000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	for _, tc := range []struct {
		name, want string
		entries    map[string][]byte
	}{
		{"traversal", "path traversal", map[string][]byte{"ok.png": png, "../bad.txt": nil}},
		{"absolute", "absolute path", map[string][]byte{"/bad.png": png}},
		{"windows", "absolute path", map[string][]byte{`C:\bad.png`: png}},
		{"backslash traversal", "path traversal", map[string][]byte{`a\..\bad.png`: png}},
		{"zip bomb", "20 MiB", map[string][]byte{"bomb.png": make([]byte, (20<<20)+1)}},
		{"pixels", "100 MP", map[string][]byte{"1.png": png, "2.png": huge}},
		{"corrupt page", "invalid cbz image", map[string][]byte{"1.png": png, "2.jpg": []byte("broken")}},
		{"malformed metadata", "invalid ComicInfo.xml", map[string][]byte{"1.png": png, "ComicInfo.xml": []byte("<ComicInfo>")}},
		{"empty", "no supported images", map[string][]byte{"ComicInfo.xml": []byte("<ComicInfo/>")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp(t)
			data, _ := app.ShelfManager().GetShelf("default_shelf")
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			archive := testutil.BuildCBZ(t, tc.entries)
			_, err := app.handlers.imports.importCBZ(data, bytes.NewReader(archive), int64(len(archive)), "comic.cbz", "", nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			books, err := os.ReadDir(filepath.Join(app.conf.Shelves[0].LibRoot, "books"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(books) != 0 {
				t.Fatalf("partial books remain: %v", books)
			}
			temporary, err := os.ReadDir(tmp)
			if err != nil || len(temporary) != 0 {
				t.Fatalf("temporary files remain: %v, %v", temporary, err)
			}
		})
	}
}

func TestCBZLocalImport(t *testing.T) {
	app := newTestApp(t)
	archive := testutil.BuildCBZ(t, map[string][]byte{"1.png": testutil.OnePixelPNG()})
	path := filepath.Join(t.TempDir(), "Fallback.CBZ")
	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatal(err)
	}
	book, err := app.handlers.imports.fromLocalPath("default_shelf", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if book.GetMeta().Title != "Fallback" || book.GetMeta().Format != "images" {
		t.Fatalf("meta = %+v", book.GetMeta())
	}
}
