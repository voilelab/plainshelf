package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
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

func TestBookCacheWorkDoesNotOverlap(t *testing.T) {
	for _, slot := range []string{"refresh", "export"} {
		t.Run(slot, func(t *testing.T) {
			s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir()})
			inProgress := &s.bookCache.refreshing
			if slot == "export" {
				inProgress = &s.bookCache.exporting
			}
			var calls atomic.Int32
			started, finish := make(chan struct{}), make(chan struct{})
			s.startBookCacheWork(inProgress, func() {
				calls.Add(1)
				close(started)
				<-finish
			})
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				close(finish)
				t.Fatal("cache work did not start")
			}
			s.startBookCacheWork(inProgress, func() { calls.Add(1) })
			close(finish)
			s.stopBackground()
			if calls.Load() != 1 || *inProgress {
				t.Fatalf("work calls = %d, claim = %v; want one call and a released claim", calls.Load(), *inProgress)
			}
		})
	}
}

func TestBookCacheClaimFailureCleanup(t *testing.T) {
	for _, slot := range []string{"refresh", "export"} {
		for _, rejected := range []bool{false, true} {
			t.Run(slot+"/rejected="+strconv.FormatBool(rejected), func(t *testing.T) {
				s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir()})
				inProgress := &s.bookCache.refreshing
				if slot == "export" {
					inProgress = &s.bookCache.exporting
				}
				if rejected {
					s.stopBackground()
				}
				var ran atomic.Bool
				s.startBookCacheWork(inProgress, func() {
					ran.Store(true)
					panic("cache work panic")
				})
				s.stopBackground()
				if *inProgress || ran.Load() == rejected {
					t.Fatalf("claim = %v, ran = %v, rejected = %v", *inProgress, ran.Load(), rejected)
				}
			})
		}
	}
}
