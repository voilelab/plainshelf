package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const closeBackgroundRounds = 300

func openShelfWithoutCleanup(t *testing.T, conf *ShelfConf) *Shelf {
	t.Helper()
	s, err := NewShelf(conf)
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := s.WaitReady(t.Context()); err != nil {
		_ = s.Close()
		t.Fatalf("WaitReady: %v", err)
	}
	return s
}

func TestCloseWaitsForBackgroundWork(t *testing.T) {
	root := t.TempDir()

	for round := range closeBackgroundRounds {
		libRoot := filepath.Join(root, "shelf-"+strconv.Itoa(round))

		s := openShelfWithoutCleanup(t, &ShelfConf{
			LibRoot:           libRoot,
			BookCheckInterval: "0s",
			BookCacheWriterID: "close-test",
			BookCacheInterval: "0s",
		})

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

func TestBackgroundPanicIsRecovered(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir()})

	ran := make(chan struct{})
	if !s.goBackground(func() {
		close(ran)
		panic("boom")
	}) {
		t.Fatal("goBackground refused to start on an open shelf")
	}
	<-ran
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
