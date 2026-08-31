package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// closeBackgroundRounds is chosen to make an unwaited goroutine visible rather
// than to be exhaustive: the window between Close returning and the shelf
// directory being deleted is short, so a single round catches the race only
// sometimes. Each round opens an empty shelf, so the whole test stays fast.
const closeBackgroundRounds = 300

// A read schedules background work — the per-book staleness check and the
// exported book cache — and both write under app/: the check takes the shelf
// lock, and flock creates app/library.lock when it is missing. A goroutine that
// outlives Close therefore writes into a shelf its owner has already closed,
// after Close released the lock that would have made the write safe to share.
//
// This is asserted the way the damage shows up: the caller deletes the shelf
// directory once Close has returned, and a straggler puts a file back under it.
// Every t.TempDir shelf in this package is that caller, which is why the
// package used to fail in cleanup with "directory not empty" on a different
// test each run.
//
// The shelves are opened without newTestShelf on purpose: its cleanup would
// close each of them a second time, after this test has already deleted the
// directory it was reading.
func TestCloseWaitsForBackgroundWork(t *testing.T) {
	root := t.TempDir()

	for round := range closeBackgroundRounds {
		libRoot := filepath.Join(root, "shelf-"+strconv.Itoa(round))

		// Both intervals elapse immediately, so every read schedules both kinds
		// of background work instead of leaving them to a timer this test would
		// have to wait out.
		s, err := NewShelf(&ShelfConf{
			LibRoot:           libRoot,
			BookCheckInterval: "0s",
			BookCacheWriterID: "close-test",
			BookCacheInterval: "0s",
		})
		if err != nil {
			t.Fatalf("round %d: NewShelf: %v", round, err)
		}
		if err := s.WaitReady(t.Context()); err != nil {
			t.Fatalf("round %d: WaitReady: %v", round, err)
		}

		if _, err := s.ListBooks(); err != nil {
			t.Fatalf("round %d: ListBooks: %v", round, err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("round %d: Close: %v", round, err)
		}

		if err := os.RemoveAll(libRoot); err != nil {
			t.Fatalf("round %d: removing the closed shelf: %v", round, err)
		}
		if _, err := os.Stat(libRoot); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("round %d: the shelf directory came back after Close: %v", round, err)
		}
	}
}
