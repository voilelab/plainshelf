package shelf

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFlockLockTimesOutWhenWriteLockIsHeld(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "library.lock")
	first := newFlockLock(lockPath, time.Second)
	second := newFlockLock(lockPath, 20*time.Millisecond)
	t.Cleanup(func() {
		_ = second.Close()
		_ = first.Unlock()
		_ = first.Close()
	})

	if err := first.Lock(); err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	if err := second.Lock(); !errors.Is(err, ErrShelfLockTimeout) {
		t.Fatalf("second Lock error = %v, want ErrShelfLockTimeout", err)
	}
	if err := second.RLock(); !errors.Is(err, ErrShelfLockTimeout) {
		t.Fatalf("second RLock error = %v, want ErrShelfLockTimeout", err)
	}
}
