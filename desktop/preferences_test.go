package main

import (
	"runtime"
	"testing"

	"github.com/voilelab/plainshelf/internal/preferences"
)

func TestDesktopPreferencesUseSharedPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("path isolation uses XDG_CONFIG_HOME")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := NewDesktopApp(&fakeNative{})

	if err := a.SetPreference("app-zoom-level", "1.25"); err != nil {
		t.Fatal(err)
	}
	path, err := preferences.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := preferences.NewStore(path, preferences.AppReader).Read()
	if err != nil || reader["app-zoom-level"] != "1.25" {
		t.Fatalf("reader preferences = %v, %v", reader, err)
	}
	got, err := a.GetPreferences()
	if err != nil || got["app-zoom-level"] != "1.25" {
		t.Fatalf("desktop preferences = %v, %v", got, err)
	}
}
