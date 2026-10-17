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
