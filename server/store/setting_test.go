package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGetSetting_NotFound(t *testing.T) {
	db := newTestDB(t)
	val, ok, err := db.GetSetting("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected key not found, got value %q", val)
	}
}

func TestSetSetting(t *testing.T) {
	db := newTestDB(t)
	if err := db.SetSetting("theme", []byte("dark")); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, ok, err := db.GetSetting("theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}
	if string(val) != "dark" {
		t.Fatalf("expected %q, got %q", "dark", val)
	}

	if err := db.SetSetting("theme", []byte("light")); err != nil {
		t.Fatalf("SetSetting (overwrite): %v", err)
	}
	val, ok, err = db.GetSetting("theme")
	if err != nil {
		t.Fatalf("GetSetting (overwrite): %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist after overwrite")
	}
	if string(val) != "light" {
		t.Fatalf("expected %q after overwrite, got %q", "light", val)
	}
}

func TestDeleteSetting(t *testing.T) {
	db := newTestDB(t)
	db.SetSetting("foo", []byte("bar"))
	if err := db.DeleteSetting("foo"); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	val, ok, err := db.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting after delete: %v", err)
	}
	if ok {
		t.Fatalf("expected key deleted, got value %q", val)
	}
}

func TestDeleteSetting_NotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteSetting("nonexistent"); err != nil {
		t.Fatalf("expected no error deleting missing key, got: %v", err)
	}
}

func TestSettingsSurviveReopen(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := db.SetSetting("kept", []byte("yes")); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := db.SetSetting("dropped", []byte("no")); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := db.DeleteSetting("dropped"); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db = newTestDBAt(t, dir)
	if val, ok, err := db.GetSetting("kept"); err != nil || !ok || string(val) != "yes" {
		t.Fatalf("kept = %q, %v, %v; want \"yes\", true, nil", val, ok, err)
	}
	if _, ok, err := db.GetSetting("dropped"); err != nil || ok {
		t.Fatalf("dropped found = %v, err %v; want gone", ok, err)
	}
}

func TestSettingsCopyValues(t *testing.T) {
	db := newTestDB(t)
	value := []byte("original")
	if err := db.SetSetting("key", value); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	got, _, err := db.GetSetting("key")
	if err != nil || string(got) != "original" {
		t.Fatalf("stored value = %q, %v", got, err)
	}
	got[0] = 'Y'
	got, _, err = db.GetSetting("key")
	if err != nil || string(got) != "original" {
		t.Fatalf("returned value aliases storage: %q, %v", got, err)
	}
}

func TestSettingsReadExistingJSON(t *testing.T) {
	dir := t.TempDir()
	raw := `{"cover_to_jpg":"dHJ1ZQ==","book_cache_writer_id":"MDEyMzQ1Njc4OWFiY2RlZg=="}`
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	db := newTestDBAt(t, dir)
	if got, ok, err := db.GetSetting("cover_to_jpg"); err != nil || !ok || string(got) != "true" {
		t.Fatalf("existing boolean = %q, %v, %v", got, ok, err)
	}
	if err := db.SetSetting("cover_to_jpg", []byte("false")); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = newTestDBAt(t, dir)
	if got, ok, err := db.GetSetting("book_cache_writer_id"); err != nil || !ok || string(got) != "0123456789abcdef" {
		t.Fatalf("existing writer ID = %q, %v, %v", got, ok, err)
	}
}

func TestSettingsRejectClosed(t *testing.T) {
	db := newTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.GetSetting("key"); err == nil {
		t.Fatal("read after close succeeded")
	}
	if err := db.SetSetting("key", []byte("value")); err == nil {
		t.Fatal("write after close succeeded")
	}
	if err := db.DeleteSetting("key"); err == nil {
		t.Fatal("delete after close succeeded")
	}
}

func TestSettingsFailedWriteRollback(t *testing.T) {
	dir := t.TempDir()
	db := newTestDBAt(t, dir)
	if err := db.SetSetting("key", []byte("original")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, settingsFile)
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file, file+".backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting("key", []byte("changed")); err == nil {
		t.Fatal("overwrite succeeded despite blocked destination")
	}
	if err := db.SetSetting("new", []byte("value")); err == nil {
		t.Fatal("insert succeeded despite blocked destination")
	}
	if err := db.DeleteSetting("key"); err == nil {
		t.Fatal("delete succeeded despite blocked destination")
	}
	if got, ok, err := db.GetSetting("key"); err != nil || !ok || string(got) != "original" {
		t.Fatalf("failed writes changed memory: %q, %v, %v", got, ok, err)
	}
	if _, ok, err := db.GetSetting("new"); err != nil || ok {
		t.Fatalf("failed insert remains in memory: %v, %v", ok, err)
	}
	backup, err := os.ReadFile(file + ".backup")
	if err != nil || !bytes.Equal(original, backup) {
		t.Fatalf("original data changed: %q, %v", backup, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file+".backup", file); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting("key", []byte("recovered")); err != nil {
		t.Fatalf("write after recovery: %v", err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "settings-*.json"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files remain: %v, %v", entries, err)
	}
}

func TestSettingsRejectInvalidFile(t *testing.T) {
	for _, raw := range []string{"null", "{", "[]", `{"key":"%%%"}`, `{"key":"YQ==","key":"Yg=="}`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, settingsFile)
			if err := os.WriteFile(file, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if db, err := New(dir); err == nil {
				db.Close()
				t.Fatal("invalid settings accepted")
			}
			got, err := os.ReadFile(file)
			if err != nil || string(got) != raw {
				t.Fatalf("invalid file changed: %q, %v", got, err)
			}
			if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			newTestDBAt(t, dir)
		})
	}
}

func TestSettingsConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	db := newTestDBAt(t, dir)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			if err := db.SetSetting(fmt.Sprint(i), []byte(fmt.Sprint(i))); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = newTestDBAt(t, dir)
	for i := range 20 {
		want := fmt.Sprint(i)
		if got, ok, err := db.GetSetting(want); err != nil || !ok || string(got) != want {
			t.Fatalf("key %s = %q, %v, %v", want, got, ok, err)
		}
	}
}

func TestSettingsDetectBadgerStore(t *testing.T) {
	if newTestDB(t).HasUnimportedBadger() {
		t.Fatal("empty store reported Badger files")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, badgerManifest), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db := newTestDBAt(t, dir)
	if !db.HasUnimportedBadger() {
		t.Fatal("Badger store without settings.json not reported")
	}
	if err := db.SetSetting("key", []byte("value")); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if newTestDBAt(t, dir).HasUnimportedBadger() {
		t.Fatal("Badger store reported after settings.json exists")
	}
}
