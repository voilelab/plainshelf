package main

import (
	"encoding/json/jsontext"
	"log"
	"slices"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/readhistory"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/readingstats"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

func (a *DesktopApp) StageReadingProgress(shelfID, bookID string, offset, at int64) {
	a.progressStager.Stage(shelfID, bookID, offset, at)
}

const maxDeviceDocumentBytes = 1 << 20

func readDeviceDocument(path string) (string, error) {
	if path == "" {
		return "", util.NewError("desktop storage is not ready")
	}

	return fsutil.ReadTextFile(path)
}

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

func (a *DesktopApp) ReadReadHistory() (string, error) {
	store := readhistory.NewStore(a.readHistoryPath)
	if a.readerNamespaceIsRealShelf() {
		return store.Read()
	}
	text, err := store.ProjectReader(readingprogress.ReaderShelfID, a.resolveUniqueBookShelf)
	if err != nil {
		log.Println("failed to project reader read history:", err)
		return store.Read()
	}
	return text, nil
}

func (a *DesktopApp) RecordReadHistory(shelfKey, bookID string) error {
	return readhistory.NewStore(a.readHistoryPath).Record(shelfKey, bookID)
}

func (a *DesktopApp) ClearReadHistory(shelfKey string) error {
	return readhistory.NewStore(a.readHistoryPath).Clear(shelfKey)
}

func (a *DesktopApp) SetReadHistoryLimit(limit int) error {
	return readhistory.NewStore(a.readHistoryPath).SetLimit(limit)
}

func (a *DesktopApp) ReadReadingProgress() (string, error) {
	if a.readingProgressSync == nil || a.readerNamespaceIsRealShelf() {
		return readDeviceDocument(a.readingProgressPath)
	}
	return projectStoredReaderProgress(a.readingProgressSync, a.resolveUniqueBookShelf)
}

func projectStoredReaderProgress(store *readingprogress.Store, resolve readingprogress.ResolveShelf) (string, error) {
	doc, raw, err := store.Read()
	if err != nil {
		return "", err
	}
	if len(doc.Shelves[readingprogress.ReaderShelfID]) == 0 {
		return raw, nil
	}

	projectedText, changed := projectionChanges(doc, resolve)
	if !changed {
		return raw, nil
	}
	return persistProjection(store, resolve, projectedText), nil
}

func projectionChanges(doc readingprogress.Document, resolve readingprogress.ResolveShelf) (string, bool) {
	projectedText, err := readingprogress.Serialize(readingprogress.Project(doc, readingprogress.ReaderShelfID, resolve))
	if err != nil {
		return "", false
	}
	currentText, err := readingprogress.Serialize(doc)
	if err != nil || projectedText == currentText {
		return "", false
	}
	return projectedText, true
}

func persistProjection(store *readingprogress.Store, resolve readingprogress.ResolveShelf, projectedText string) string {
	final, err := store.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.Project(disk, readingprogress.ReaderShelfID, resolve)
	})
	if err != nil {
		log.Println("failed to persist projected reading progress:", err)
		return projectedText
	}
	finalText, err := readingprogress.Serialize(final)
	if err != nil {
		return projectedText
	}
	return finalText
}

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

func (a *DesktopApp) resolveUniqueBookShelf(bookID string) (string, bool) {
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
				return "", false
			}
			match = shelfData.ID
		}
	}
	if match == "" {
		return "", false
	}
	return match, true
}

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

func (a *DesktopApp) ReadReadingStats() (string, error) {
	store := readingstats.NewStore(a.readingStatsPath)
	if a.readerNamespaceIsRealShelf() {
		return store.Read()
	}
	text, err := store.ProjectReader(readingprogress.ReaderShelfID, a.resolveUniqueBookShelf)
	if err != nil {
		log.Println("failed to project reader reading stats:", err)
		return store.Read()
	}
	return text, nil
}

func (a *DesktopApp) AddReadingSeconds(key, date string, seconds float64) error {
	return readingstats.NewStore(a.readingStatsPath).Add(key, date, seconds, time.Now)
}
