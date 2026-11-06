package main

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

type shownMessage struct {
	kind    dialogKind
	title   string
	message string
}

type emittedEvent struct {
	name string
	data any
}

type fakeNative struct {
	mu       sync.Mutex
	messages []shownMessage
	scripts  []string
	urls     []string
	quits    int
	copied   []string
	shown    int
	events   []emittedEvent
	tray     []menuItem
}

func (f *fakeNative) openFiles(string, []fileFilter) ([]string, error) { return nil, nil }
func (f *fakeNative) openDirectory(string) (string, error)             { return "", nil }

func (f *fakeNative) saveFile(string, string, []fileFilter) (string, error) {
	return "", nil
}

func (f *fakeNative) showMessage(kind dialogKind, title, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, shownMessage{kind: kind, title: title, message: message})
	return nil
}

func (f *fakeNative) openURL(rawURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.urls = append(f.urls, rawURL)
}

func (f *fakeNative) execJS(script string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts = append(f.scripts, script)
}

func (f *fakeNative) quit() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quits++
}

func (f *fakeNative) copyText(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copied = append(f.copied, text)
}

func (f *fakeNative) showWindow() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown++
}

func (f *fakeNative) emit(name string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, emittedEvent{name: name, data: data})
}

func (f *fakeNative) setTrayMenu(items []menuItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tray = items
}

func (f *fakeNative) trayItems() []menuItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tray
}

func TestDomReadyStartupErrorShowsAndQuits(t *testing.T) {
	native := &fakeNative{}
	app := NewDesktopApp(native)
	app.startupErr = errors.New("store locked")

	app.DomReady(t.Context())

	if len(native.messages) != 1 {
		t.Fatalf("shown messages = %+v, want one error dialog", native.messages)
	}
	got := native.messages[0]
	if got.kind != errorDialog || got.title != "PlainShelf failed to start" || !strings.Contains(got.message, "store locked") {
		t.Fatalf("startup failure dialog = %+v", got)
	}
	if native.quits != 1 {
		t.Fatalf("quit calls = %d, want 1", native.quits)
	}
}

func TestDomReadyShelfErrorsWarnWithoutQuit(t *testing.T) {
	native := &fakeNative{}
	app := NewDesktopApp(native)
	app.shelfOpenErrs = []error{errors.New("shelf \"Remote\": unreachable")}

	app.DomReady(t.Context())

	if len(native.messages) != 1 || native.messages[0].kind != warningDialog ||
		!strings.Contains(native.messages[0].message, "unreachable") {
		t.Fatalf("shown messages = %+v, want one shelf warning", native.messages)
	}
	if native.quits != 0 {
		t.Fatalf("quit calls = %d, want 0", native.quits)
	}
}

func TestDomReadyCleanStartShowsNothing(t *testing.T) {
	native := &fakeNative{}
	NewDesktopApp(native).DomReady(t.Context())

	if len(native.messages) != 0 || native.quits != 0 {
		t.Fatalf("clean start showed %+v and quit %d times", native.messages, native.quits)
	}
}

func TestOpenExternalURLUsesNative(t *testing.T) {
	native := &fakeNative{}
	app := NewDesktopApp(native)

	if err := app.OpenExternalURL(" https://example.com/a "); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenExternalURL("file:///etc/passwd"); err == nil {
		t.Fatal("file URL accepted")
	}
	if len(native.urls) != 1 || native.urls[0] != "https://example.com/a" {
		t.Fatalf("opened URLs = %q, want only the https one", native.urls)
	}
}
