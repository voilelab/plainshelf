package apitest

import (
	neturl "net/url"
	"strings"
)

const SettingPath = "/api/setting/cover_to_jpg"

func ShelfIDURL(shelfID string, elem ...string) string {
	return strings.Join(append([]string{"/api/shelves", shelfID}, elem...), "/")
}

func ShelfURL(elem ...string) string {
	return ShelfIDURL(DefaultShelfID, elem...)
}

func SecondShelfBooksURL() string {
	return ShelfIDURL(SecondShelfID, "books")
}

func ScansURL() string {
	return ShelfURL("scans")
}

func BookCacheExportURL() string {
	return ShelfURL("book-cache-exports")
}

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

func DownloadsURL() string {
	return ShelfIDURL(VirtualShelfID, "downloads")
}

func BookDownloadURL(bookID string) string {
	return ShelfIDURL(VirtualShelfID, "books", bookID, "download")
}

func VirtualBookURL(bookID string, elem ...string) string {
	return ShelfIDURL(VirtualShelfID, append([]string{"books", bookID}, elem...)...)
}

func SourceURL(bookID, sourceID string, elem ...string) string {
	return BookURL(bookID, append([]string{"sources", sourceID}, elem...)...)
}

func AssetURL(bookID, sourceID, name string) string {
	return SourceURL(bookID, sourceID, "assets") + "/" + name
}

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

func TrashBooksURL(elem ...string) string {
	return ShelfURL(append([]string{"trash", "books"}, elem...)...)
}

func EmptyTrashURL() string {
	return ShelfURL("trash", "empty")
}

func FolderTransfersURL() string {
	return ShelfURL("folder-transfers")
}

func TaskChainURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID
}

func TaskChainCancelURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID + "/cancel"
}

func LogURL(elem ...string) string {
	return strings.Join(append([]string{"/api/logs"}, elem...), "/")
}

func SettingURL(key string) string {
	return "/api/setting/" + key
}
