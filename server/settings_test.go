package server

import (
	"io"
	"log/slog"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
)

func TestShowNSFWFailsClosed(t *testing.T) {
	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	settings := &settings{
		Logger: &logutil.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		db:     db,
	}
	if err := db.SetSetting(settingKeyShowNSFW, []byte("true")); err != nil {
		t.Fatal(err)
	}
	if !settings.readShowNSFWFailClosed() {
		t.Fatal("stored true did not enable visibility")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	if _, _, err := db.GetSetting(settingKeyShowNSFW); err == nil {
		t.Skip("store backend does not fail reads after Close")
	}
	if settings.readShowNSFWFailClosed() {
		t.Fatal("store read failure enabled visibility")
	}
}
