package main

import (
	"errors"
	"runtime"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

func newNotifyTestApp(t *testing.T, native *fakeNative) *DesktopApp {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("path isolation uses XDG_CONFIG_HOME")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return NewDesktopApp(native)
}

func TestNotifyImportInBackground(t *testing.T) {
	native := &fakeNative{}
	newNotifyTestApp(t, native).NotifyImportFinished("Import finished", "Imported 3 books")

	if len(native.notified) != 1 || native.notified[0] != "Import finished|Imported 3 books" {
		t.Fatalf("notifications = %q, want one by default", native.notified)
	}
}

func TestNotifyImportSkippedWhenFocused(t *testing.T) {
	native := &fakeNative{focused: true}
	newNotifyTestApp(t, native).NotifyImportFinished("Import finished", "Imported 3 books")

	if len(native.notified) != 0 {
		t.Fatalf("notifications = %q, want none while focused", native.notified)
	}
}

func TestNotifyImportSkippedWhenDisabled(t *testing.T) {
	native := &fakeNative{}
	app := newNotifyTestApp(t, native)
	if err := app.SetPreference(importNotificationsKey, "false"); err != nil {
		t.Fatal(err)
	}
	app.NotifyImportFinished("Import finished", "Imported 3 books")

	if len(native.notified) != 0 {
		t.Fatalf("notifications = %q, want none when turned off", native.notified)
	}
}

type fakeNotifyBackend struct {
	startErr  error
	granted   bool
	answer    bool
	starts    int
	checks    int
	requests  int
	delivered []notifications.NotificationOptions
}

func (f *fakeNotifyBackend) start() error {
	f.starts++
	return f.startErr
}

func (f *fakeNotifyBackend) authorized() (bool, error) {
	f.checks++
	return f.granted, nil
}

func (f *fakeNotifyBackend) requestAccess() (bool, error) {
	f.requests++
	f.granted = f.answer
	return f.answer, nil
}

func (f *fakeNotifyBackend) deliver(o notifications.NotificationOptions) error {
	f.delivered = append(f.delivered, o)
	return nil
}

func TestLazyNotifierAsksOnFirstSend(t *testing.T) {
	backend := &fakeNotifyBackend{answer: true}
	n := &lazyNotifier{backend: backend}
	if backend.starts != 0 || backend.requests != 0 {
		t.Fatal("notifier touched the backend before the first send")
	}

	for range 2 {
		if err := n.send("t", "b"); err != nil {
			t.Fatal(err)
		}
	}
	if backend.starts != 1 || backend.requests != 1 || len(backend.delivered) != 2 {
		t.Fatalf("starts=%d requests=%d delivered=%d, want 1/1/2",
			backend.starts, backend.requests, len(backend.delivered))
	}
	if backend.delivered[0].ID == backend.delivered[1].ID {
		t.Fatalf("notification IDs repeat: %q", backend.delivered[0].ID)
	}
}

func TestLazyNotifierDeniedStopsAsking(t *testing.T) {
	backend := &fakeNotifyBackend{answer: false}
	n := &lazyNotifier{backend: backend}

	for range 3 {
		if err := n.send("t", "b"); err != nil {
			t.Fatalf("denied permission reported %v, want silence", err)
		}
	}
	if backend.requests != 1 || backend.checks != 1 || len(backend.delivered) != 0 {
		t.Fatalf("checks=%d requests=%d delivered=%d, want 1/1/0",
			backend.checks, backend.requests, len(backend.delivered))
	}
}

func TestLazyNotifierStartFailureSticks(t *testing.T) {
	backend := &fakeNotifyBackend{startErr: errors.New("no bundle identifier")}
	n := &lazyNotifier{backend: backend}

	for range 2 {
		if err := n.send("t", "b"); err == nil {
			t.Fatal("send succeeded without a started notifier")
		}
	}
	if backend.starts != 1 || backend.checks != 0 {
		t.Fatalf("starts=%d checks=%d, want 1/0", backend.starts, backend.checks)
	}
}
