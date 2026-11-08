package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

func TestForwardedLaunchOpensWindow(t *testing.T) {
	app, native := newTestApp(t)
	first := launchBook(t, app, "")
	const nextBook = "nextbook"
	argv := []string{"/Applications/PlainShelfReader.app/Contents/MacOS/PlainShelfReader",
		"-book", writeBookPackage(t, nextBook), "-shelf", "shelf-real", "-section", "2"}

	if err := app.OpenForwardedLaunch(argv, "/"); err != nil {
		t.Fatalf("OpenForwardedLaunch: %v", err)
	}
	if len(native.created) != 2 || len(native.reloads) != 0 || len(native.errors) != 0 {
		t.Fatalf("native calls = %+v, want a second window and no reload or error", native)
	}
	wantBoot(t, app, first, readerapi.ShelfID, testBookID, noLaunchSection)
	wantBoot(t, app, native.created[1], "shelf-real", nextBook, 2)
}

func TestForwardedRelativeBookUsesWorkDir(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})
	dir := writeBookPackage(t, "relbook")

	err := app.OpenForwardedLaunch([]string{"reader", "-book", filepath.Base(dir)}, filepath.Dir(dir))
	if err != nil {
		t.Fatalf("OpenForwardedLaunch: %v", err)
	}
	wantBoot(t, app, win, readerapi.ShelfID, "relbook", noLaunchSection)
	if !slices.Equal(native.reloads, []uint{win}) || !slices.Equal(native.focuses, []uint{win}) {
		t.Fatalf("native calls = %+v, want the empty window reloaded and focused", native)
	}
}

func TestForwardedLaunchWithoutBookFocuses(t *testing.T) {
	app, native := newTestApp(t)
	win := launchBook(t, app, "launch-shelf")

	if err := app.OpenForwardedLaunch([]string{"reader"}, "/"); err != nil {
		t.Fatalf("OpenForwardedLaunch: %v", err)
	}
	wantBoot(t, app, win, "launch-shelf", testBookID, noLaunchSection)
	if len(native.created) != 1 || len(native.reloads) != 0 || !slices.Equal(native.focuses, []uint{0}) {
		t.Fatalf("native calls = %+v, want only a focus on the front window", native)
	}
}

func TestForwardedLaunchBadArgsRejected(t *testing.T) {
	app, native := newTestApp(t)
	win := launchBook(t, app, "launch-shelf")

	if err := app.OpenForwardedLaunch([]string{"reader", "-unknown"}, "/"); err == nil {
		t.Fatal("accepted an unknown forwarded flag")
	}
	wantBoot(t, app, win, "launch-shelf", testBookID, noLaunchSection)
	if len(native.created) != 1 || len(native.reloads) != 0 {
		t.Fatalf("native calls = %+v, want the original window untouched", native)
	}
}

func TestForwardedNonBookKeepsCurrentBook(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.bookpkg")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	metaDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(metaDir, "book.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"missing path":          filepath.Join(t.TempDir(), "absent.bookpkg"),
		"plain file":            file,
		"folder without meta":   t.TempDir(),
		"book.json is a folder": metaDir,
		"relative path":         "book.bookpkg",
	}
	for name, path := range tests {
		t.Run(name, func(t *testing.T) {
			app, native := newTestApp(t)
			win := launchBookAt(t, app, "launch-shelf", testBookID, 3)

			err := app.OpenFinderFile(path)
			if !errors.Is(err, errNotBookPackage) {
				t.Fatalf("OpenFinderFile(%q) = %v, want errNotBookPackage", path, err)
			}
			wantBoot(t, app, win, "launch-shelf", testBookID, 3)
			if !slices.Equal(native.errorWins, []uint{0}) || len(native.reloads) != 0 || len(native.created) != 1 {
				t.Fatalf("native calls = %+v, want one error on the front window and no new window", native)
			}
		})
	}
}

func TestFinderFileOpensStandaloneWindow(t *testing.T) {
	app, native := newTestApp(t)
	first := launchBookAt(t, app, "launch-shelf", testBookID, 3)

	if err := app.OpenFinderFile(writeBookPackage(t, "finderbook")); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	if len(native.created) != 2 {
		t.Fatalf("windows = %v, want two", native.created)
	}
	wantBoot(t, app, first, "launch-shelf", testBookID, 3)
	wantBoot(t, app, native.created[1], readerapi.ShelfID, "finderbook", noLaunchSection)
}

func TestSameBookFocusesExistingWindow(t *testing.T) {
	app, native := newTestApp(t)
	first := launchBook(t, app, "shelf-real")
	if err := app.OpenFinderFile(writeBookPackage(t, "otherbook")); err != nil {
		t.Fatal(err)
	}

	if err := app.OpenFinderFile(writeBookPackage(t, testBookID)); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	if len(native.created) != 2 || native.focuses[len(native.focuses)-1] != first {
		t.Fatalf("windows %v, focuses %v; want no third window and window %d focused", native.created, native.focuses, first)
	}
	wantBoot(t, app, first, "shelf-real", testBookID, noLaunchSection)
}

func TestWindowReadyWaitsForInFlightOpen(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})
	dir := writeBookPackage(t, "finderbook")

	app.openMu.Lock()
	done := make(chan struct{})
	go func() {
		app.windowReady(win)
		close(done)
	}()
	select {
	case <-done:
		app.openMu.Unlock()
		t.Fatal("windowReady did not wait for the in-flight open")
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := app.library.Open(dir); err != nil {
		t.Fatal(err)
	}
	app.fillEmptyWindow(win, app.newWindowState("", "finderbook", noLaunchSection))
	app.openMu.Unlock()
	<-done

	if len(native.prompts) != 0 {
		t.Fatalf("windowReady prompted %v after a Finder open, want none", native.prompts)
	}
}
