package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

// TestExampleConfigDecodes guards the config keys this tool re-declares instead
// of importing from server.AppConf. If someone renames one there, the shipped
// example config stops feeding this tool and the failure would otherwise be a
// silent "no shelves in the config".
func TestExampleConfigDecodes(t *testing.T) {
	conf, err := loadConf(filepath.Join("..", "plainshelf-srv", "conf", "config.yaml"))
	if err != nil {
		t.Fatalf("loadConf: %v", err)
	}

	if len(conf.AppConf.Shelves) != 1 {
		t.Fatalf("got %d shelves, want the example config's one", len(conf.AppConf.Shelves))
	}
	if got := conf.AppConf.Shelves[0].ID; got != "default_shelf" {
		t.Errorf("shelf id = %q, want %q", got, "default_shelf")
	}
	if got := conf.AppConf.Shelves[0].LibRoot; got != "./shelf" {
		t.Errorf("lib_root = %q, want %q", got, "./shelf")
	}
	if got := conf.AppConf.StorePath; got != "./store" {
		t.Errorf("store_path = %q, want %q", got, "./store")
	}
}

func TestSelectShelf(t *testing.T) {
	one := &migrateConf{}
	one.AppConf.Shelves = []*shelf.ShelfConfWithID{{ID: "only"}}

	two := &migrateConf{}
	two.AppConf.Shelves = []*shelf.ShelfConfWithID{{ID: "first"}, {ID: "second"}}

	t.Run("a single shelf needs no flag", func(t *testing.T) {
		selected, err := selectShelf(one, "")
		if err != nil {
			t.Fatalf("selectShelf: %v", err)
		}
		if selected.ID != "only" {
			t.Errorf("selected %q, want %q", selected.ID, "only")
		}
	})

	t.Run("several shelves need the flag", func(t *testing.T) {
		if _, err := selectShelf(two, ""); err == nil {
			t.Fatalf("selectShelf accepted an ambiguous config")
		}
	})

	t.Run("an unknown id is refused", func(t *testing.T) {
		if _, err := selectShelf(two, "third"); err == nil {
			t.Fatalf("selectShelf accepted an unknown shelf id")
		}
	})

	t.Run("no shelves at all is refused", func(t *testing.T) {
		if _, err := selectShelf(&migrateConf{}, ""); err == nil {
			t.Fatalf("selectShelf accepted a config with no shelves")
		}
	})
}

func TestResolveDefaultSplit(t *testing.T) {
	configured := &shelf.SplitConfig{Type: shelf.SplitTypeLineCount, LineCount: 100}

	newStore := func(t *testing.T, value string) *store.DB {
		t.Helper()
		db, err := store.New(t.TempDir())
		if err != nil {
			t.Fatalf("store.New: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		if value != "" {
			if err := db.SetSetting(settingKeyDefaultSplitConfig, []byte(value)); err != nil {
				t.Fatalf("SetSetting: %v", err)
			}
		}
		return db
	}

	t.Run("the stored setting wins", func(t *testing.T) {
		db := newStore(t, `{"type":"regex","regex":"^Chapter"}`)
		got := resolveDefaultSplit(db, configured)
		if got.Type != shelf.SplitTypeRegex || got.Regex != "^Chapter" {
			t.Errorf("got %+v, want the stored regex", got)
		}
	})

	t.Run("no stored setting falls back to the config", func(t *testing.T) {
		got := resolveDefaultSplit(newStore(t, ""), configured)
		if !reflect.DeepEqual(got, *configured) {
			t.Errorf("got %+v, want %+v", got, *configured)
		}
	})

	t.Run("an unparsable stored setting counts as absent", func(t *testing.T) {
		got := resolveDefaultSplit(newStore(t, "not json"), configured)
		if !reflect.DeepEqual(got, *configured) {
			t.Errorf("got %+v, want the configured value %+v", got, *configured)
		}
	})

	t.Run("no store and no config means no split", func(t *testing.T) {
		got := resolveDefaultSplit(nil, nil)
		if got.Type != shelf.SplitTypeNone {
			t.Errorf("got %+v, want the empty split", got)
		}
	})

	t.Run("the stored shape matches what the server writes", func(t *testing.T) {
		// The server stores the setting as the JSON encoding of the same type,
		// so a round trip must survive.
		want := shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{4, 9}}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		got := resolveDefaultSplit(newStore(t, string(raw)), nil)
		if got.Type != want.Type || len(got.Boundaries) != len(want.Boundaries) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestSplitBookIDs(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{"", 0},
		{"  ", 0},
		{"a", 1},
		{"a,b", 2},
		{" a , b , ", 2},
	}

	for _, test := range tests {
		if got := splitBookIDs(test.value); len(got) != test.want {
			t.Errorf("splitBookIDs(%q) = %v, want %d ids", test.value, got, test.want)
		}
	}
}
