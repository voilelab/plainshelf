package main

import (
	"testing"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

func TestReaderStageThenCloseRoundTrips(t *testing.T) {
	app, _ := newTestApp(t)
	win := launchBook(t, app, "")

	app.StageReadingProgress(readerapi.ShelfID, testBookID, 4321, 1000)
	app.windowClosed(win)

	if v := diskDoc(t, app).Shelves[readerapi.ShelfID][testBookID].Offset; v != 4321 {
		t.Fatalf("restored offset = %d, want 4321 (staged position must survive the close)", v)
	}
	if app.library.Has(testBookID) {
		t.Fatal("closed window's book is still open")
	}
}

func TestClosingOneWindowKeepsOther(t *testing.T) {
	app, _ := newTestApp(t)
	first := launchBook(t, app, "")
	second := launchBookAt(t, app, "shelf-real", "otherbook", noLaunchSection)
	app.StageReadingProgress(readerapi.ShelfID, testBookID, 100, 1000)
	app.StageReadingProgress("shelf-real", "otherbook", 200, 1000)

	app.windowClosed(first)

	disk := diskDoc(t, app)
	if v := disk.Shelves[readerapi.ShelfID][testBookID].Offset; v != 100 {
		t.Fatalf("closed window's offset = %d, want 100", v)
	}
	if _, ok := disk.Shelves["shelf-real"]["otherbook"]; ok {
		t.Fatal("closing one window wrote the other window's staged progress")
	}
	if app.library.Has(testBookID) || !app.library.Has("otherbook") {
		t.Fatal("closing one window must close only its own book")
	}
	wantBoot(t, app, second, "shelf-real", "otherbook", noLaunchSection)

	app.windowClosed(second)
	if v := diskDoc(t, app).Shelves["shelf-real"]["otherbook"].Offset; v != 200 {
		t.Fatalf("last window's offset = %d, want 200", v)
	}
}
