package main

import (
	"testing"
	"time"

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

func TestReopenDuringCloseKeepsBook(t *testing.T) {
	app, native := newTestApp(t)
	dir := writeBookPackage(t, testBookID)
	first := app.Launch(launchArgs{bookPath: dir, section: noLaunchSection})
	launchBookAt(t, app, "", "otherbook", noLaunchSection)
	app.StageReadingProgress(readerapi.ShelfID, testBookID, 100, 1000)
	release := holdProgressLock(t, app.progressStore)

	closed := make(chan struct{})
	go func() {
		app.windowClosed(first)
		close(closed)
	}()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, open := app.window(first); !open {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("closing window was never removed")
		}
	}

	if err := app.OpenFinderFile(dir); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	release()
	<-closed

	reopened := native.created[len(native.created)-1]
	wantBoot(t, app, reopened, readerapi.ShelfID, testBookID, noLaunchSection)
	if !app.library.Has(testBookID) {
		t.Fatal("the old window's close released the reopened window's book")
	}
}

func TestReopenOpenBookKeepsOneHold(t *testing.T) {
	app, _ := newTestApp(t)
	dir := writeBookPackage(t, testBookID)
	first := app.Launch(launchArgs{bookPath: dir, section: noLaunchSection})

	if err := app.OpenFinderFile(dir); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	app.windowClosed(first)

	if app.library.Has(testBookID) {
		t.Fatal("focusing an open book left an extra hold on its package")
	}
}
