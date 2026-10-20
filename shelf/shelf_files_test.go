package shelf

import "testing"

func TestIsShelfFile(t *testing.T) {
	for name, want := range map[string]bool{
		".":                                true,
		"books":                            true,
		"books/f/a.bookpkg/book.json":      true,
		"trash/books/a.bookpkg/trash.json": true,
		"shelf.json":                       true,
		"app":                              true,
		"app/book-cache-abc.json":          true,
		"books/a.bookpkg/assets/notes.tmp": true,
		"books/a.bookpkg/assets/a.tmp.txt": true,
		"books/a.bookpkg/assets/notes.abcdefg.tmp":           true,
		"books/a.bookpkg/sources/.20260101-000000-abcde.tmp": true,
		"books/a.bookpkg/sources/.-abcdef.tmp":               true,

		"books/a.bookpkg/book.json.aB3dE6gH.tmp":                         false,
		"books/a.bookpkg/sources/.20260101-000000-aB3dE6.tmp":            false,
		"books/a.bookpkg/sources/.20260101-000000-aB3dE6.tmp/meta.json":  false,
		"trash/books/a.bookpkg/sources/.20260101-000000-aB3dE6.tmp/x.md": false,

		"shelf.json/x":                      false,
		"app/library.lock":                  false,
		"app/scan-cache.json":               false,
		"app/book-cache-abc.json.x1.tmp":    false,
		"app/tmp":                           false,
		"app/tmp/book-cache-abc.json":       false,
		".trash/books/a.bookpkg/trash.json": false,
		"stray.txt":                         false,
		"":                                  false,
	} {
		if got := IsShelfFile(name); got != want {
			t.Errorf("IsShelfFile(%q) = %v, want %v", name, got, want)
		}
	}
}
