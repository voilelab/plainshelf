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

func TestCloseWaitsForBackgroundWork(t *testing.T) {
	root := t.TempDir()

	for round := range closeBackgroundRounds {
		libRoot := filepath.Join(root, "shelf-"+strconv.Itoa(round))

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
