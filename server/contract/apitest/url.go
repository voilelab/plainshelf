package apitest

import (
	neturl "net/url"
	"strings"
)

// The builders below assemble every route server/routes.go registers, in the
// order that file lists them. They keep the repeated /api/shelves/<id> prefix
// in one place while leaving each route's own shape spelled out at the call
// site, since that shape is what a contract test is pinning. Reading this file
// top to bottom is reading the API surface.

// SettingPath is a representative route for the gate tests: it needs no
// imported book, since reading it answers 200 and updating it 204, so one path
// exercises both sides of the token gate.
const SettingPath = "/api/setting/cover_to_jpg"

// Shelf API

func ShelfIDURL(shelfID string, elem ...string) string {
	return strings.Join(append([]string{"/api/shelves", shelfID}, elem...), "/")
}

// ShelfURL addresses the single shelf the contract tests configure.
func ShelfURL(elem ...string) string {
	return ShelfIDURL(DefaultShelfID, elem...)
}

// SecondShelfBooksURL lists the books on the shelf WithSecondShelf adds.
func SecondShelfBooksURL() string {
	return ShelfIDURL(SecondShelfID, "books")
}

func ScansURL() string {
	return ShelfURL("scans")
}

func BookCacheExportURL() string {
	return ShelfURL("book-cache-exports")
}

// Book API

func BooksURL() string {
	return ShelfURL("books")
}

func BookURL(bookID string, elem ...string) string {
	return ShelfURL(append([]string{"books", bookID}, elem...)...)
}

func BookBatchURL() string {
	return ShelfURL("book-batches")
}

func ContentStatsURL() string {
	return ShelfURL("content-stat-refreshes")
}

func SourceFingerprintsURL() string {
	return ShelfURL("source-fingerprints")
}

func FingerprintStatusURL() string {
	return ShelfURL("fingerprints", "status")
}

func SimilarURL() string {
	return ShelfURL("books", "similar")
}

func BookCopiesURL(bookID string) string {
	return BookURL(bookID, "copies")
}

func BookTransfersURL(bookID string) string {
	return BookURL(bookID, "transfers")
}

// Source API

func SourceURL(bookID, sourceID string, elem ...string) string {
	return BookURL(bookID, append([]string{"sources", sourceID}, elem...)...)
}

// AssetURL addresses one file under a source's assets/ directory. The name is
// used verbatim so a test can address an already-escaped or otherwise unusual
// name and see what the route does with it.
func AssetURL(bookID, sourceID, name string) string {
	return SourceURL(bookID, sourceID, "assets") + "/" + name
}

// AssetsBundleURL addresses a source's batch assets.zip endpoint, naming the
// files to pack with repeated `name` query parameters. With no names it packs
// the whole assets/ directory.
func AssetsBundleURL(bookID, sourceID string, names ...string) string {
	base := SourceURL(bookID, sourceID, "assets.zip")
	if len(names) == 0 {
		return base
	}
	query := neturl.Values{}
	for _, name := range names {
		query.Add("name", name)
	}
	return base + "?" + query.Encode()
}

// Trash and folder API

func TrashBooksURL(elem ...string) string {
	return ShelfURL(append([]string{"trash", "books"}, elem...)...)
}

func EmptyTrashURL() string {
	return ShelfURL("trash", "empty")
}

func FolderTransfersURL() string {
	return ShelfURL("folder-transfers")
}

// Task API

func TaskChainURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID
}

func TaskChainCancelURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID + "/cancel"
}

// Log API

func LogURL(elem ...string) string {
	return strings.Join(append([]string{"/api/logs"}, elem...), "/")
}

// Setting API

func SettingURL(key string) string {
	return "/api/setting/" + key
}
