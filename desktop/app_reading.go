package main

import (
	"encoding/json/jsontext"
	"log"
	"slices"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// StageReadingProgress is bound to the frontend, which calls it on every reading
// position change so the desktop shell holds the latest position in memory and
// can write it on close. It only stages; the disk write happens in beforeClose.
func (a *DesktopApp) StageReadingProgress(shelfID, bookID string, offset, at int64) {
	a.progressStager.Stage(shelfID, bookID, offset, at)
}

// Reading history, progress, and stats are per-device state: they never reach the
// server, and on the desktop they must not depend on WebView storage either
// (clearing site data or a WebView profile change would take them with it).
// Each is stored as a JSON file next to shelves.json instead.
//
// The document formats belong to the frontend (frontend/src/storage/*), which
// owns the single implementation of merging and trimming; this side only checks
// that what it is handed is JSON of a sane size, and persists it.
const maxDeviceDocumentBytes = 1 << 20

// readDeviceDocument returns the stored document, or an empty string when this
// device has not stored one yet.
func readDeviceDocument(path string) (string, error) {
	if path == "" {
		return "", util.NewError("desktop storage is not ready")
	}

	return fsutil.ReadTextFile(path)
}

// writeDeviceDocument replaces the stored document, once it is JSON of a sane
// size. The write itself is atomic; see fsutil.WriteTextFileAtomic.
func writeDeviceDocument(path, label, doc string) error {
	if path == "" {
		return util.NewError("desktop storage is not ready")
	}
	if len(doc) > maxDeviceDocumentBytes {
		return util.Errorf("%s document is too large: %d bytes", label, len(doc))
	}
	if !jsontext.Value(doc).IsValid() {
		return util.Errorf("%s document is not valid JSON", label)
	}

	return fsutil.WriteTextFileAtomic(path, ".device_document-*.json", doc)
}

// ReadReadHistory reports an empty string when this device has stored none.
func (a *DesktopApp) ReadReadHistory() (string, error) {
	return readDeviceDocument(a.readHistoryPath)
}

func (a *DesktopApp) WriteReadHistory(doc string) error {
	return writeDeviceDocument(a.readHistoryPath, "read history", doc)
}

// ReadReadingProgress is the projection trigger: the standalone reader writes
// into the same file under its synthetic "book" shelf id, and that progress is
// folded onto the real shelves it belongs to before the document reaches the
// frontend. Running it on every read is what a returning-from-the-reader user
// causes; see projectStoredReaderProgress for why it is cheap when there is
// nothing to fold.
func (a *DesktopApp) ReadReadingProgress() (string, error) {
	if a.readingProgressSync == nil || a.readerNamespaceIsRealShelf() {
		return readDeviceDocument(a.readingProgressPath)
	}
	return projectStoredReaderProgress(a.readingProgressSync, a.resolveBookShelf)
}

// projectStoredReaderProgress folds any standalone-reader progress onto the real
// shelves resolve reports. The common case — nothing to fold — takes no lock and
// writes nothing; only genuinely new reader progress triggers a locked
// read-modify-write.
func projectStoredReaderProgress(store *readingprogress.Store, resolve readingprogress.ResolveShelf) (string, error) {
	doc, raw, err := store.Read()
	if err != nil {
		return "", err
	}
	if len(doc.Shelves[readingprogress.ReaderShelfID]) == 0 {
		return raw, nil
	}

	projected := readingprogress.Project(doc, readingprogress.ReaderShelfID, resolve)
	projectedText, err := readingprogress.Serialize(projected)
	if err != nil {
		return raw, nil //nolint:nilerr // convenience state: fall back to the stored view
	}
	currentText, err := readingprogress.Serialize(doc)
	if err != nil || projectedText == currentText {
		return raw, nil //nolint:nilerr // nothing new to fold in
	}

	// There is new reader progress to persist. Fold it in under the store's lock
	// (re-reading fresh, so a concurrent reader write is not lost). Best-effort:
	// still return the projected view if the write fails.
	final, err := store.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.Project(disk, readingprogress.ReaderShelfID, resolve)
	})
	if err != nil {
		log.Println("failed to persist projected reading progress:", err)
		return projectedText, nil
	}
	finalText, err := readingprogress.Serialize(final)
	if err != nil {
		return projectedText, nil //nolint:nilerr // convenience state: return the projected view
	}
	return finalText, nil
}

// WriteReadingProgress records the desktop's reading-progress entries. The
// incoming write is merged newest-wins per book against the on-disk document
// re-read under the store's lock, so a standalone reader's "book" entries and a
// desktop-launched reader's real-shelf entries are both preserved unless the
// desktop's write is more recent — a desktop write never clobbers reader progress
// by recency, and its own resets (timestamped tombstones) still win.
func (a *DesktopApp) WriteReadingProgress(doc string) error {
	if a.readingProgressSync == nil {
		return writeDeviceDocument(a.readingProgressPath, "reading progress", doc)
	}

	incoming, err := readingprogress.ParseStrict(doc)
	if err != nil {
		return err
	}
	_, err = a.readingProgressSync.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeNewest(disk, incoming)
	})
	return err
}

// resolveBookShelf reports which real shelf holds a stable book id.
//
// Book ids are only unique within a shelf, so a copied or legacy package can
// carry one id in two shelves: an ambiguous id is left unresolved rather than
// projected onto whichever shelf came first in the map-ordered scan. A shelf
// still initializing is treated as "not here", and a later read re-attempts the
// projection once it is ready.
func (a *DesktopApp) resolveBookShelf(bookID string) (string, bool) {
	if a.app == nil {
		return "", false
	}
	manager := a.app.ShelfManager()
	if manager == nil {
		return "", false
	}
	match := ""
	for _, shelfData := range manager.GetAllShelves() {
		if _, err := shelfData.GetBook(bookID); err == nil {
			if match != "" {
				return "", false // ambiguous: more than one shelf holds this id
			}
			match = shelfData.ID
		}
	}
	if match == "" {
		return "", false
	}
	return match, true
}

// readerNamespaceIsRealShelf disables reader projection when a real shelf uses
// the id the standalone reader stores progress under: that shelf must not be
// blocked from saving its own progress. New shelves cannot take the id (see
// generateDesktopShelfID); this guards a config predating that rule.
func (a *DesktopApp) readerNamespaceIsRealShelf() bool {
	if a.app == nil {
		return false
	}
	manager := a.app.ShelfManager()
	if manager == nil {
		return false
	}
	return slices.ContainsFunc(manager.GetAllShelves(), func(shelfData shelf.ShelfData) bool {
		return shelfData.ID == readingprogress.ReaderShelfID
	})
}

// ReadReadingStats reports an empty string when this device has stored none.
func (a *DesktopApp) ReadReadingStats() (string, error) {
	return readDeviceDocument(a.readingStatsPath)
}

// WriteReadingStats replaces the stored reading-stats document.
func (a *DesktopApp) WriteReadingStats(doc string) error {
	return writeDeviceDocument(a.readingStatsPath, "reading stats", doc)
}
