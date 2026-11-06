package main

import (
	"net/url"
	"strconv"
	"sync"

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
	name := readerWindowName(shelfID, bookID)
	a.windows.opened(name)
	a.native.openReaderWindow(readerWindow{
		name:    name,
		title:   title,
		url:     readerWindowURL(shelfID, bookID, section),
		onReuse: func() { a.progressStager.PersistBookOnClose(shelfID, bookID) },
		onClose: func() { a.closeReaderWindow(name, shelfID, bookID) },
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

func (a *DesktopApp) closeReaderWindow(name, shelfID, bookID string) {
	a.progressStager.PersistBookOnClose(shelfID, bookID)
	a.native.emit(readingDataChangedEvent)
	if a.windows.closed(name) {
		a.quitApp()
	}
}

func (a *DesktopApp) closeMainWindow() (keepWindow bool) {
	if a.shuttingDown.Load() {
		return false
	}
	if a.windows.hideMain() {
		a.native.hideMain()
		return true
	}
	a.quitApp()
	return false
}

func (a *DesktopApp) showLibrary() {
	a.windows.showMain()
	a.native.showMain()
}

func (a *DesktopApp) quitApp() {
	if a.shuttingDown.Load() {
		return
	}
	a.Shutdown()
	a.native.quit()
}

type windowSet struct {
	mu         sync.Mutex
	readers    map[string]struct{}
	mainHidden bool
}

func (s *windowSet) opened(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readers == nil {
		s.readers = map[string]struct{}{}
	}
	s.readers[name] = struct{}{}
}

func (s *windowSet) closed(name string) (lastWindow bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.readers, name)
	return s.mainHidden && len(s.readers) == 0
}

func (s *windowSet) hideMain() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mainHidden = len(s.readers) > 0
	return s.mainHidden
}

func (s *windowSet) showMain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mainHidden = false
}
