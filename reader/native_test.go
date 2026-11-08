package main

import (
	"context"
	"slices"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeNative struct {
	dir       string
	prompts   []uint
	errors    []string
	errorWins []uint
	reloads   []uint
	focuses   []uint
	created   []uint
	current   uint
}

func (f *fakeNative) newWindow(prepare func(win uint)) uint {
	win := uint(len(f.created) + 1)
	prepare(win)
	f.created = append(f.created, win)
	f.current = win
	return win
}

func (f *fakeNative) currentWindow() uint { return f.current }

func (f *fakeNative) openDirectory(win uint, _ string) (string, error) {
	f.prompts = append(f.prompts, win)
	return f.dir, nil
}

func (f *fakeNative) showError(win uint, title, _ string) error {
	f.errors = append(f.errors, title)
	f.errorWins = append(f.errorWins, win)
	return nil
}

func (f *fakeNative) reload(win uint) { f.reloads = append(f.reloads, win) }

func (f *fakeNative) focus(win uint) { f.focuses = append(f.focuses, win) }

type fakeWindowID uint

func (f fakeWindowID) ID() uint { return uint(f) }

func windowContext(t *testing.T, win uint) context.Context {
	t.Helper()
	return context.WithValue(t.Context(), application.WindowKey, fakeWindowID(win))
}

func TestOpenBookPackageNonBookShowsError(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})
	native.dir = t.TempDir()

	if _, err := app.openBookPackageIn(win); err == nil {
		t.Fatal("openBookPackageIn accepted a folder without book.json")
	}
	if !slices.Equal(native.errors, []string{"Cannot open this folder"}) || !slices.Equal(native.errorWins, []uint{win}) {
		t.Fatalf("shown errors = %q on %v, want the not-a-book dialog on window %d", native.errors, native.errorWins, win)
	}
	if len(native.reloads) != 0 {
		t.Fatalf("reloads = %v, want none", native.reloads)
	}
}

func TestOpenBookPackageFillsEmptyWindow(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})
	native.dir = writeBookPackage(t, testBookID)

	bookID, err := app.openBookPackageIn(win)
	if err != nil {
		t.Fatal(err)
	}
	if bookID != testBookID || !slices.Equal(native.reloads, []uint{win}) || len(native.created) != 1 {
		t.Fatalf("book %q, reloads %v, windows %v; want the empty window reloaded", bookID, native.reloads, native.created)
	}
	if boot := app.bootConfig(win); boot.BookID != testBookID {
		t.Fatalf("boot = %+v, want %s", boot, testBookID)
	}
}

func TestOpenBookPackageCancelDoesNothing(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})

	if bookID, err := app.openBookPackageIn(win); bookID != "" || err != nil {
		t.Fatalf("openBookPackageIn = %q, %v; want empty, nil", bookID, err)
	}
	if len(native.reloads) != 0 || len(native.errors) != 0 || len(native.created) != 1 {
		t.Fatalf("cancel changed windows: %+v", native)
	}
}

func TestOpenBookPackageUsesCallerWindow(t *testing.T) {
	app, native := newTestApp(t)
	app.Launch(launchArgs{section: noLaunchSection})
	second := app.Launch(launchArgs{section: noLaunchSection})
	native.dir = t.TempDir()

	if _, err := app.OpenBookPackage(windowContext(t, second)); err == nil {
		t.Fatal("OpenBookPackage accepted a folder without book.json")
	}
	if !slices.Equal(native.prompts, []uint{second}) || !slices.Equal(native.errorWins, []uint{second}) {
		t.Fatalf("prompts %v, errors on %v; want both on window %d", native.prompts, native.errorWins, second)
	}
}

func TestWindowReadyPromptsForBookOnce(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{section: noLaunchSection})

	app.windowReady(win)
	app.windowReady(win)

	if !slices.Equal(native.prompts, []uint{win}) {
		t.Fatalf("book prompts = %v, want one on window %d", native.prompts, win)
	}
}

func TestWindowReadyWithBookSkipsPrompt(t *testing.T) {
	app, native := newTestApp(t)
	win := launchBook(t, app, "")

	app.windowReady(win)

	if len(native.prompts) != 0 {
		t.Fatalf("book prompts = %v, want none for a window with a book", native.prompts)
	}
}

func TestApplicationMenuOpenBook(t *testing.T) {
	application.New(application.Options{Name: "PlainShelf Reader"})
	app, native := newTestApp(t)
	win := launchBook(t, app, "")
	specs := applicationMenu(app)

	for _, spec := range specs {
		for _, item := range spec.items {
			if item.label != "Open Book…" {
				continue
			}
			if item.accel != "CmdOrCtrl+O" {
				t.Fatalf("Open Book accelerator = %q, want CmdOrCtrl+O", item.accel)
			}
			item.action()
			if !slices.Equal(native.prompts, []uint{win}) {
				t.Fatalf("Open Book prompts = %v, want one on the current window %d", native.prompts, win)
			}
			if _, err := toWailsMenu(specs); err != nil {
				t.Fatalf("toWailsMenu: %v", err)
			}
			return
		}
	}
	t.Fatal("Open Book menu item not found")
}
