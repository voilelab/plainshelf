package store

import (
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

	// Changing a setting the user already saved must replace the stored value,
	// not leave the original in place.
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

// Settings outlive the process: the browser build reopens its store on every page load.
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
