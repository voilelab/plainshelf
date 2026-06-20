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
}

func TestSetSetting_Overwrite(t *testing.T) {
	db := newTestDB(t)
	db.SetSetting("lang", []byte("en"))
	if err := db.SetSetting("lang", []byte("ja")); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, ok, err := db.GetSetting("lang")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}
	if string(val) != "ja" {
		t.Fatalf("expected %q, got %q", "ja", val)
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

func TestSetSetting_MultipleKeys(t *testing.T) {
	db := newTestDB(t)
	settings := map[string]string{"a": "1", "b": "2", "c": "3"}
	for k, v := range settings {
		if err := db.SetSetting(k, []byte(v)); err != nil {
			t.Fatalf("SetSetting %q: %v", k, err)
		}
	}
	for k, want := range settings {
		got, ok, err := db.GetSetting(k)
		if err != nil {
			t.Fatalf("GetSetting %q: %v", k, err)
		}
		if !ok {
			t.Fatalf("key %q not found", k)
		}
		if string(got) != want {
			t.Fatalf("key %q: expected %q, got %q", k, want, got)
		}
	}
}
