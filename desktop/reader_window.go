package main

import (
	"net/url"
	"strconv"

	"github.com/voilelab/plainshelf/internal/util"
)

const readingDataChangedEvent = "plainshelf:reading-data-changed"

type readerWindow struct {
	name    string
	title   string
	url     string
	onReuse func()
	onClose func()
}

func readerWindowName(shelfID, bookID string) string {
	return "reader:" + url.PathEscape(shelfID) + "/" + url.PathEscape(bookID)
}

func readerWindowURL(shelfID, bookID string, section int) string {
	query := url.Values{"shelf": {shelfID}}
	if section >= 0 {
		query.Set("section", strconv.Itoa(section))
	}
	return "/reader/" + url.PathEscape(bookID) + "?" + query.Encode()
}

func (a *DesktopApp) OpenReaderWindow(shelfID, bookID string, section int) error {
	title, err := a.bookTitle(shelfID, bookID)
	if err != nil {
		return err
	}
	a.native.openReaderWindow(readerWindow{
		name:    readerWindowName(shelfID, bookID),
		title:   title,
		url:     readerWindowURL(shelfID, bookID, section),
		onReuse: func() { a.progressStager.PersistBookOnClose(shelfID, bookID) },
		onClose: func() { a.closeReaderWindow(shelfID, bookID) },
	})
	return nil
}

func (a *DesktopApp) bookTitle(shelfID, bookID string) (string, error) {
	if a.app == nil {
		return "", util.NewError("desktop backend app instance is nil")
	}
	shelfData, ok := a.app.ShelfManager().GetShelf(shelfID)
	if !ok {
		return "", util.Errorf("shelf %q not found", shelfID)
	}
	book, err := shelfData.GetBook(bookID)
	if err != nil {
		return "", util.Errorf("opening reader window: %w", err)
	}
	if title := book.Title(); title != "" {
		return title, nil
	}
	return "PlainShelf", nil
}

func (a *DesktopApp) closeReaderWindow(shelfID, bookID string) {
	a.progressStager.PersistBookOnClose(shelfID, bookID)
	a.native.emit(readingDataChangedEvent)
}

func (a *DesktopApp) closeMainWindow() {
	a.Shutdown()
	a.native.quit()
}
