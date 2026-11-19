package shelf

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
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
	if !s.goBackground(func(context.Context) {
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
			s.startBookCacheWork(inProgress, func(context.Context) {
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
			s.startBookCacheWork(inProgress, func(context.Context) { calls.Add(1) })
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
				s.startBookCacheWork(inProgress, func(context.Context) {
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

type slowScanFS struct {
	fsutil.FS
	started chan struct{}
	release chan struct{}
	first   sync.Once
	reads   atomic.Int32
}

func (f *slowScanFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != booksFolder && filepath.Dir(name) == booksFolder {
		f.reads.Add(1)
		f.first.Do(func() {
			close(f.started)
			<-f.release
		})
		time.Sleep(150 * time.Millisecond)
	}
	return f.FS.ReadDir(name)
}

func TestCloseCancelsInitialScan(t *testing.T) {
	dir := t.TempDir()
	conf := &ShelfConf{LibRoot: dir, BookCacheWriterID: "close-test"}
	seed := openShelfWithoutCleanup(t, conf)
	for i := range 10 {
		folder := FolderPath{strconv.Itoa(i)}
		if _, err := seed.NewBook(folder, "Book "+folder[0]); err != nil {
			t.Fatal(err)
		}
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	bookCachePath := filepath.Join(dir, appFolder, seed.bookCacheFileName())
	scanCachePath := filepath.Join(dir, appFolder, scanCacheFileName)
	for _, name := range []string{bookCachePath, scanCachePath} {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	ageShelfDirs(t, dir, time.Minute)

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	slow := &slowScanFS{FS: fsutil.NewRootFS(root), started: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(slow.release) })
	defer release()
	settings, err := parseShelfSettings(conf)
	if err != nil {
		t.Fatal(err)
	}
	s, err := startShelf(conf, settings, slow, nil, newNoneLock(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	var closeErr error
	closeShelf := sync.OnceFunc(func() {
		go func() {
			closeErr = s.Close()
			close(closed)
		}()
	})
	t.Cleanup(func() {
		release()
		closeShelf()
		<-closed
	})
	select {
	case <-slow.started:
	case <-time.After(5 * time.Second):
		t.Fatal("initial scan did not reach the slow directory")
	}

	timeout := time.After(time.Second)
	closeShelf()
	select {
	case <-s.backgroundCtx.Done():
	case <-timeout:
		t.Fatal("Close did not cancel the initial scan within one second")
	}
	select {
	case <-closed:
		t.Fatal("Close returned while the filesystem read was in flight")
	default:
	}
	release()
	select {
	case <-closed:
	case <-timeout:
		t.Fatal("Close did not return within one second")
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if err := s.WaitReady(t.Context()); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady = %v, want cancellation", err)
	}
	if got := slow.reads.Load(); got != 1 {
		t.Errorf("scan read %d slow directories, want only the in-flight read", got)
	}
	for _, name := range []string{bookCachePath, scanCachePath} {
		if _, err := os.Stat(name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("incomplete scan cache at %s: stat returned %v", name, err)
		}
	}

	reopened := newTestShelf(t, conf)
	if got := len(reopened.listBookListingsFromCache()); got != 10 {
		t.Errorf("reopened shelf has %d books, want 10", got)
	}
}
