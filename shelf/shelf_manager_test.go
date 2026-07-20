package shelf

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestShelfManagerLifecycle(t *testing.T) {
	sm := NewShelfManager()
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = sm.Close()
		}
	})

	if err := sm.AddShelf(ShelfConfWithID{}); err == nil {
		t.Fatal("AddShelf with an empty ID succeeded, want error")
	}

	firstRoot := t.TempDir()
	if err := sm.AddShelf(ShelfConfWithID{
		ID: "primary",
		ShelfConf: ShelfConf{
			LibRoot:      firstRoot,
			LockMode:     "none",
			ScanInterval: "1h",
		},
	}); err != nil {
		t.Fatalf("AddShelf(primary): %v", err)
	}

	primary, ok := sm.GetShelf("primary")
	if !ok {
		t.Fatal("GetShelf(primary) did not find the added shelf")
	}
	if primary.Name != "primary" {
		t.Fatalf("default shelf name = %q, want %q", primary.Name, "primary")
	}
	if err := primary.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady(primary): %v", err)
	}

	if err := sm.AddShelf(ShelfConfWithID{
		ID:        "primary",
		ShelfConf: ShelfConf{LibRoot: t.TempDir(), LockMode: "none"},
	}); err == nil {
		t.Fatal("AddShelf accepted a duplicate ID")
	}

	if err := sm.AddShelf(ShelfConfWithID{
		ID:   "secondary",
		Name: "Secondary Shelf",
		ShelfConf: ShelfConf{
			LibRoot:  t.TempDir(),
			LockMode: "none",
		},
	}); err != nil {
		t.Fatalf("AddShelf(secondary): %v", err)
	}
	secondary, ok := sm.GetShelf("secondary")
	if !ok {
		t.Fatal("GetShelf(secondary) did not find the added shelf")
	}
	if err := secondary.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady(secondary): %v", err)
	}

	if got := len(sm.GetAllShelves()); got != 2 {
		t.Fatalf("GetAllShelves length = %d, want 2", got)
	}

	if err := sm.UpdateShelf("missing", "Missing", "1m"); err == nil {
		t.Fatal("UpdateShelf accepted an unknown shelf ID")
	}
	if err := sm.UpdateShelf("primary", "Changed", "not-a-duration"); err == nil {
		t.Fatal("UpdateShelf accepted an invalid scan interval")
	}
	if primary.Name != "primary" {
		t.Fatalf("invalid update changed name to %q", primary.Name)
	}

	if err := sm.UpdateShelf("primary", "Main Shelf", "2m"); err != nil {
		t.Fatalf("UpdateShelf(primary): %v", err)
	}
	if primary.Name != "Main Shelf" {
		t.Fatalf("updated shelf name = %q, want %q", primary.Name, "Main Shelf")
	}
	primary.bookCache.RLock()
	interval := primary.bookCache.scanInterval
	primary.bookCache.RUnlock()
	if interval != 2*time.Minute {
		t.Fatalf("updated scan interval = %v, want %v", interval, 2*time.Minute)
	}

	if err := sm.RemoveShelf("missing"); err == nil {
		t.Fatal("RemoveShelf accepted an unknown shelf ID")
	}
	if err := sm.RemoveShelf("primary"); err != nil {
		t.Fatalf("RemoveShelf(primary): %v", err)
	}
	if _, ok := sm.GetShelf("primary"); ok {
		t.Fatal("GetShelf(primary) found a removed shelf")
	}

	if err := sm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	closed = true
}

func TestShelfConfigurationValidation(t *testing.T) {
	tests := []struct {
		name string
		conf *ShelfConf
		want string
	}{
		{name: "nil", conf: nil, want: "cannot be nil"},
		{name: "scan interval", conf: &ShelfConf{LibRoot: t.TempDir(), ScanInterval: "bad"}, want: "invalid scan interval"},
		{name: "lock timeout", conf: &ShelfConf{LibRoot: t.TempDir(), LockTimeout: "bad"}, want: "invalid lock timeout"},
		{name: "book check interval", conf: &ShelfConf{LibRoot: t.TempDir(), BookCheckInterval: "bad"}, want: "invalid book check interval"},
		{name: "lock mode", conf: &ShelfConf{LibRoot: t.TempDir(), LockMode: "invalid"}, want: "unknown lock_mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewShelf(tt.conf)
			if err == nil {
				if s != nil {
					_ = s.Close()
				}
				t.Fatal("NewShelf succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewShelf error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
