package main

import (
	"runtime"
	"testing"

	"github.com/voilelab/plainshelf/internal/preferences"
)

func TestReaderPreferencesShareDesktopFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("path isolation uses XDG_CONFIG_HOME")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := NewReaderApp(nil, nil, "shelf-real", noLaunchSection)
	t.Cleanup(func() { app.Shutdown(t.Context()) })

	if err := app.SetPreference("reader-font-size", "26"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetPreference("plainshelf.shelf", "reader-shelf"); err != nil {
		t.Fatal(err)
	}

	path, err := preferences.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := preferences.NewStore(path, preferences.AppDesktop).Read()
	if err != nil {
		t.Fatal(err)
	}
	if desktop["reader-font-size"] != "26" {
		t.Errorf("desktop reader-font-size = %q, want 26", desktop["reader-font-size"])
	}
	if _, ok := desktop["plainshelf.shelf"]; ok {
		t.Errorf("reader shelf leaked into desktop: %v", desktop)
	}
	own, err := app.GetPreferences()
	if err != nil || own["plainshelf.shelf"] != "reader-shelf" {
		t.Fatalf("reader preferences = %v, %v", own, err)
	}
}
