package shelf

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
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

	if err := sm.UpdateShelf(ShelfConfWithID{ID: "missing", Name: "Missing"}); err == nil {
		t.Fatal("UpdateShelf accepted an unknown shelf ID")
	}
	if err := sm.UpdateShelf(ShelfConfWithID{
		ID:        "primary",
		Name:      "Changed",
		ShelfConf: ShelfConf{LibRoot: firstRoot, LockMode: "none", ScanInterval: "not-a-duration"},
	}); err == nil {
		t.Fatal("UpdateShelf accepted an invalid scan interval")
	}
	if primary.Name != "primary" {
		t.Fatalf("invalid update changed name to %q", primary.Name)
	}
	if err := sm.UpdateShelf(ShelfConfWithID{
		ID:        "primary",
		Name:      "Moved",
		ShelfConf: ShelfConf{LibRoot: t.TempDir(), LockMode: "none"},
	}); err == nil {
		t.Fatal("UpdateShelf accepted a different lib_root")
	}

	if err := sm.UpdateShelf(ShelfConfWithID{
		ID:        "primary",
		Name:      "Main Shelf",
		ShelfConf: ShelfConf{LibRoot: firstRoot, LockMode: "none", ScanInterval: "2m"},
	}); err != nil {
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

// read_only is the one shelf setting UpdateShelf cannot apply to the live
// shelf, and the one whose change must not be one-way: a shelf opened
// read-only has to be able to become writable again without a restart.
func TestShelfManagerUpdateShelfTogglesReadOnly(t *testing.T) {
	libRoot := t.TempDir()
	bookID := seedReadOnlyShelf(t, libRoot)

	sm := NewShelfManager()
	t.Cleanup(func() { _ = sm.Close() })

	writable := ShelfConfWithID{
		ID:        "primary",
		Name:      "Primary",
		ShelfConf: ShelfConf{LibRoot: libRoot, LockMode: "none"},
	}
	if err := sm.AddShelf(writable); err != nil {
		t.Fatalf("AddShelf(primary): %v", err)
	}

	readOnly := writable
	readOnly.ReadOnly = true
	if err := sm.UpdateShelf(readOnly); err != nil {
		t.Fatalf("UpdateShelf to read-only: %v", err)
	}

	s := waitShelfReady(t, sm, "primary")
	if !s.ReadOnly() {
		t.Fatal("ReadOnly() = false after the shelf was updated to read_only")
	}
	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks on the read-only shelf: %v", err)
	}
	if len(books) != 1 || books[0].ID() != bookID {
		t.Fatalf("ListBooks returned %d books, want the seeded %q", len(books), bookID)
	}
	if err := s.DeleteBook(bookID); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("DeleteBook error = %v, want %v", err, fsutil.ErrReadOnly)
	}

	if err := sm.UpdateShelf(writable); err != nil {
		t.Fatalf("UpdateShelf back to writable: %v", err)
	}

	s = waitShelfReady(t, sm, "primary")
	if s.ReadOnly() {
		t.Fatal("ReadOnly() = true after the shelf was updated back to writable")
	}
	if err := s.DeleteBook(bookID); err != nil {
		t.Fatalf("DeleteBook after read_only was turned off: %v", err)
	}
}

// A read_only change closes the shelf before it opens the new one, so a
// configuration that does not open would otherwise leave the shelf gone. The
// previous one is opened again instead, and the shelf keeps working.
func TestShelfManagerUpdateShelfRestoresShelfWhenReopenFails(t *testing.T) {
	libRoot := t.TempDir()

	sm := NewShelfManager()
	t.Cleanup(func() { _ = sm.Close() })

	writable := ShelfConfWithID{
		ID:        "primary",
		Name:      "Primary",
		ShelfConf: ShelfConf{LibRoot: libRoot, LockMode: "none"},
	}
	if err := sm.AddShelf(writable); err != nil {
		t.Fatalf("AddShelf(primary): %v", err)
	}
	waitShelfReady(t, sm, "primary")

	// A read-only shelf is never created, so a lib_root that is not there is an
	// error rather than a new shelf - the writable configuration recreates it.
	if err := os.RemoveAll(libRoot); err != nil {
		t.Fatalf("RemoveAll(libRoot): %v", err)
	}

	readOnly := writable
	readOnly.ReadOnly = true
	if err := sm.UpdateShelf(readOnly); err == nil {
		t.Fatal("UpdateShelf to read_only on a missing lib_root succeeded, want an error")
	}

	s := waitShelfReady(t, sm, "primary")
	if s.ReadOnly() {
		t.Fatal("ReadOnly() = true after a failed update, want the previous configuration back")
	}
	if _, err := s.ListBooks(); err != nil {
		t.Fatalf("ListBooks after a failed update: %v", err)
	}
}

func waitShelfReady(t *testing.T, sm *ShelfManager, id string) *ShelfData {
	t.Helper()

	s, ok := sm.GetShelf(id)
	if !ok {
		t.Fatalf("GetShelf(%q) did not find the shelf", id)
	}
	if err := s.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady(%q): %v", id, err)
	}
	return s
}
