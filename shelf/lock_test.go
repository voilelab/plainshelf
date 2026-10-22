package shelf

import (
	"errors"
	"path/filepath"
	"slices"
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

type transferTestLock struct {
	ShelfLock
	name   string
	events *[]string
	fail   bool
}

func (l *transferTestLock) Lock() error {
	*l.events = append(*l.events, l.name+" write")
	if l.fail {
		return ErrShelfLockTimeout
	}
	return nil
}

func (l *transferTestLock) RLock() error {
	*l.events = append(*l.events, l.name+" read")
	if l.fail {
		return ErrShelfLockTimeout
	}
	return nil
}

func (l *transferTestLock) Unlock() error {
	*l.events = append(*l.events, l.name+" unlock")
	return nil
}

func TestTransferLocksReleaseOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		sourceFail, targetFail bool
		want                   []string
	}{
		{"success", false, false, []string{"source read", "target write", "target unlock", "source unlock"}},
		{"source failure", true, false, []string{"source read"}},
		{"target failure", false, true, []string{"source read", "target write", "source unlock"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			source := &Shelf{shelfLock: &transferTestLock{name: "source", events: &events, fail: tc.sourceFail}}
			target := &Shelf{shelfLock: &transferTestLock{name: "target", events: &events, fail: tc.targetFail}}
			locked, unlock, err := lockForTransfer(source, target)
			if tc.sourceFail || tc.targetFail {
				if !errors.Is(err, ErrShelfLockTimeout) {
					t.Fatalf("lock error = %v", err)
				}
			} else {
				if err != nil || locked.Shelf != target {
					t.Fatalf("locked target = %v, error = %v", locked.Shelf, err)
				}
				unlock()
			}
			if !slices.Equal(events, tc.want) {
				t.Errorf("lock events = %v, want %v", events, tc.want)
			}
		})
	}
}
