package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

func TestForwardedLaunchOpensBook(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithBook(t, "")
	app.native = native
	const nextBook = "nextbook"
	argv := []string{"/Applications/PlainShelfReader.app/Contents/MacOS/PlainShelfReader",
		"-book", writeBookPackage(t, nextBook), "-shelf", "shelf-real", "-section", "2"}

	if err := app.OpenForwardedLaunch(argv, "/"); err != nil {
		t.Fatalf("OpenForwardedLaunch: %v", err)
	}
	boot := app.BootConfig()
	if boot.BookID != nextBook || boot.ShelfID != "shelf-real" || boot.Section == nil || *boot.Section != 2 {
		t.Fatalf("forwarded boot config = %+v, want next book on shelf-real at section 2", boot)
	}
	if native.reloads != 1 || native.focuses != 1 || len(native.errors) != 0 {
		t.Fatalf("native calls = %+v, want one reload and focus and no error", native)
	}
}

func TestForwardedRelativeBookUsesWorkDir(t *testing.T) {
	app := newReaderAppWithNative(t, &fakeNative{})
	dir := writeBookPackage(t, "relbook")

	err := app.OpenForwardedLaunch([]string{"reader", "-book", filepath.Base(dir)}, filepath.Dir(dir))
	if err != nil || app.library.BookID() != "relbook" {
		t.Fatalf("OpenForwardedLaunch = %v, book %q", err, app.library.BookID())
	}
}

func TestForwardedLaunchWithoutBookFocuses(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithBook(t, "launch-shelf")
	app.native = native

	if err := app.OpenForwardedLaunch([]string{"reader"}, "/"); err != nil {
		t.Fatalf("OpenForwardedLaunch: %v", err)
	}
	if app.library.BookID() != testBookID || native.reloads != 0 || native.focuses != 1 {
		t.Fatalf("book %q, native %+v; want original book, focus only", app.library.BookID(), native)
	}
}

func TestForwardedLaunchBadArgsRejected(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithBook(t, "launch-shelf")
	app.native = native

	if err := app.OpenForwardedLaunch([]string{"reader", "-unknown"}, "/"); err == nil {
		t.Fatal("accepted an unknown forwarded flag")
	}
	if app.library.BookID() != testBookID || native.reloads != 0 {
		t.Fatalf("book %q, reloads %d; want original book untouched", app.library.BookID(), native.reloads)
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
			native := &fakeNative{}
			app := newReaderAppWithBook(t, "launch-shelf")
			app.native = native
			app.launchSection = 3

			err := app.OpenFinderFile(path)
			if !errors.Is(err, errNotBookPackage) {
				t.Fatalf("OpenFinderFile(%q) = %v, want errNotBookPackage", path, err)
			}
			boot := app.BootConfig()
			if boot.BookID != testBookID || boot.ShelfID != "launch-shelf" || boot.Section == nil || *boot.Section != 3 {
				t.Fatalf("boot config = %+v, want original book and launch state", boot)
			}
			if len(native.errors) != 1 || native.reloads != 0 {
				t.Fatalf("native calls = %+v, want one error dialog and no reload", native)
			}
		})
	}
}

func TestFinderFileOpensStandalone(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithBook(t, "launch-shelf")
	app.native = native

	if err := app.OpenFinderFile(writeBookPackage(t, "finderbook")); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	boot := app.BootConfig()
	if boot.BookID != "finderbook" || boot.ShelfID != readerapi.ShelfID || boot.Section != nil {
		t.Fatalf("Finder boot config = %+v, want standalone book without section", boot)
	}
	if native.reloads != 1 || native.focuses != 1 {
		t.Fatalf("native calls = %+v, want one reload and focus", native)
	}
}

func TestDomReadyWaitsForInFlightOpen(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithNative(t, native)
	dir := writeBookPackage(t, "finderbook")

	app.openMu.Lock()
	done := make(chan struct{})
	go func() {
		app.DomReady(context.Background())
		close(done)
	}()
	select {
	case <-done:
		app.openMu.Unlock()
		t.Fatal("DomReady did not wait for the in-flight open")
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := app.library.Open(dir); err != nil {
		t.Fatal(err)
	}
	app.openMu.Unlock()
	<-done

	if native.prompts != 0 {
		t.Fatalf("DomReady prompted %d times after a Finder open, want 0", native.prompts)
	}
}
