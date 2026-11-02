package main

import (
	"testing"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

type fakeNative struct {
	dir     string
	prompts int
	errors  []string
	reloads int
}

func (f *fakeNative) openDirectory(string) (string, error) {
	f.prompts++
	return f.dir, nil
}

func (f *fakeNative) showError(title, _ string) error {
	f.errors = append(f.errors, title)
	return nil
}

func (f *fakeNative) reloadApp() { f.reloads++ }

func newReaderAppWithNative(t *testing.T, native nativeUI) *ReaderApp {
	t.Helper()
	library := readerapi.NewLibrary(nil)
	t.Cleanup(func() { library.Close() })
	return &ReaderApp{native: native, library: library, shelfID: readerapi.ShelfID}
}

func TestOpenBookPackageNonBookShowsError(t *testing.T) {
	native := &fakeNative{dir: t.TempDir()}
	app := newReaderAppWithNative(t, native)

	if _, err := app.OpenBookPackage(); err == nil {
		t.Fatal("OpenBookPackage accepted a folder without book.json")
	}
	if len(native.errors) != 1 || native.errors[0] != "Cannot open this folder" {
		t.Fatalf("shown errors = %q, want the not-a-book dialog", native.errors)
	}
	if native.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", native.reloads)
	}
}

func TestOpenBookPackageBookReloads(t *testing.T) {
	native := &fakeNative{dir: writeBookPackage(t, testBookID)}
	app := newReaderAppWithNative(t, native)

	bookID, err := app.OpenBookPackage()
	if err != nil {
		t.Fatal(err)
	}
	if bookID != testBookID || native.reloads != 1 || len(native.errors) != 0 {
		t.Fatalf("book %q, reloads %d, errors %q", bookID, native.reloads, native.errors)
	}
}

func TestOpenBookPackageCancelDoesNothing(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithNative(t, native)

	if bookID, err := app.OpenBookPackage(); bookID != "" || err != nil {
		t.Fatalf("OpenBookPackage = %q, %v; want empty, nil", bookID, err)
	}
	if native.reloads != 0 || len(native.errors) != 0 {
		t.Fatalf("cancel reloaded %d times and showed %q", native.reloads, native.errors)
	}
}

func TestDomReadyPromptsForBookOnce(t *testing.T) {
	native := &fakeNative{}
	app := newReaderAppWithNative(t, native)

	app.DomReady(t.Context())
	app.DomReady(t.Context())

	if native.prompts != 1 {
		t.Fatalf("book prompts = %d, want 1", native.prompts)
	}
}

func TestApplicationMenuOpenBook(t *testing.T) {
	native := &fakeNative{}
	specs := applicationMenu(newReaderAppWithNative(t, native))

	for _, spec := range specs {
		for _, item := range spec.items {
			if item.label != "Open Book…" {
				continue
			}
			if item.accel != "CmdOrCtrl+O" {
				t.Fatalf("Open Book accelerator = %q, want CmdOrCtrl+O", item.accel)
			}
			item.action()
			if native.prompts != 1 {
				t.Fatalf("Open Book prompts = %d, want 1", native.prompts)
			}
			if _, err := toWailsMenu(specs); err != nil {
				t.Fatalf("toWailsMenu: %v", err)
			}
			return
		}
	}
	t.Fatal("Open Book menu item not found")
}
