package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// countingFS records every call a shelf makes through it, standing in for a
// remote file tree where each call is a request.
type countingFS struct {
	inner fsutil.ReadFS

	mu    sync.Mutex
	calls []string
}

func (c *countingFS) record(op, name string) {
	c.mu.Lock()
	c.calls = append(c.calls, op+" "+name)
	c.mu.Unlock()
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.record("Open", name)
	return c.inner.Open(name)
}

func (c *countingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	c.record("ReadDir", name)
	return c.inner.ReadDir(name)
}

func (c *countingFS) Stat(name string) (fs.FileInfo, error) {
	c.record("Stat", name)
	return c.inner.Stat(name)
}

// take returns the calls recorded so far and starts a new count.
func (c *countingFS) take() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	calls := c.calls
	c.calls = nil
	return calls
}

// booksCalls is the part of calls that reached into books/: what costs a
// request per book on a remote shelf.
func booksCalls(calls []string) []string {
	var out []string
	for _, call := range calls {
		if _, name, _ := strings.Cut(call, " "); name == booksFolder || strings.HasPrefix(name, booksFolder+"/") {
			out = append(out, call)
		}
	}
	return out
}

// seedExportedShelf creates a local shelf holding the given titles, one of them
// in a folder, and exports its book cache the way a server sharing it would.
func seedExportedShelf(t *testing.T, titles ...string) string {
	t.Helper()

	libRoot := t.TempDir()
	s, err := NewShelf(&ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer s.Close()
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	for i, title := range titles {
		folder := FolderPath{}
		if i%2 == 1 {
			folder = FolderPath{"Fiction"}
		}
		if _, err := s.NewBook(folder, title); err != nil {
			t.Fatalf("NewBook(%q): %v", title, err)
		}
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	return libRoot
}

// openRemoteShelf opens libRoot through NewShelfFromFS behind a countingFS.
func openRemoteShelf(t *testing.T, libRoot string, conf ShelfConf) (*Shelf, *countingFS) {
	t.Helper()

	rt, err := os.OpenRoot(libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })

	counter := &countingFS{inner: fsutil.NewRootFS(rt)}
	s, err := NewShelfFromFS(&conf, counter, "")
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s, counter
}

func listedTitles(t *testing.T, s *Shelf) []string {
	t.Helper()

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	var titles []string
	for _, book := range books {
		titles = append(titles, book.Title())
	}
	slices.Sort(titles)
	return titles
}

func TestRemoteShelfListsFromExportedCacheWithoutReadingBooks(t *testing.T) {
	counts := map[int]int{}
	for _, n := range []int{2, 12} {
		titles := make([]string, n)
		for i := range titles {
			titles[i] = "Book " + string(rune('A'+i))
		}
		libRoot := seedExportedShelf(t, titles...)

		s, counter := openRemoteShelf(t, libRoot, ShelfConf{})
		got := listedTitles(t, s)
		if _, err := s.GetAllFolders(); err != nil {
			t.Fatalf("GetAllFolders: %v", err)
		}

		if !slices.Equal(got, titles) {
			t.Errorf("n=%d: listed %v, want %v", n, got, titles)
		}
		calls := counter.take()
		if reached := booksCalls(calls); len(reached) > 0 {
			t.Errorf("n=%d: opening and listing reached into books/: %v", n, reached)
		}
		counts[n] = len(calls)
	}

	if counts[2] != counts[12] {
		t.Errorf("calls to open and list = %v by book count, want the same for every size", counts)
	}
}

func TestRemoteShelfRefreshRereadsOnlyTheExport(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune", "Solaris")
	s, counter := openRemoteShelf(t, libRoot, ShelfConf{ScanInterval: "1ms", BookCheckInterval: "1ms"})
	counter.take()

	time.Sleep(5 * time.Millisecond)
	listedTitles(t, s)
	time.Sleep(5 * time.Millisecond)
	listedTitles(t, s)

	calls := counter.take()
	if reached := booksCalls(calls); len(reached) > 0 {
		t.Errorf("refreshing reached into books/: %v", reached)
	}
	if !slices.Contains(calls, "ReadDir "+appFolder) {
		t.Errorf("refresh calls = %v, want a listing of %s/", calls, appFolder)
	}

	// The exporter adds a book and exports again. Its walk starts in the same
	// second as the first one in a test, so the timestamp is moved forward.
	local := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if _, err := local.NewBook(FolderPath{}, "Hyperion"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := local.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.Timestamp += 10
	writeRawBookCache(t, libRoot, path.Base(cachePath), cache)

	time.Sleep(5 * time.Millisecond)
	if got, want := listedTitles(t, s), []string{"Dune", "Hyperion", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("after a new export, listed %v, want %v", got, want)
	}
	if reached := booksCalls(counter.take()); len(reached) > 0 {
		t.Errorf("picking up the new export reached into books/: %v", reached)
	}
}

func TestRemoteShelfWithoutExportWalksOnlyOnOpen(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune", "Solaris")
	if err := os.RemoveAll(path.Join(libRoot, appFolder)); err != nil {
		t.Fatalf("remove app/: %v", err)
	}

	s, counter := openRemoteShelf(t, libRoot, ShelfConf{ScanInterval: "1ms", BookCheckInterval: "1ms"})
	if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	if len(booksCalls(counter.take())) == 0 {
		t.Error("opening without an export did not walk books/")
	}

	time.Sleep(5 * time.Millisecond)
	listedTitles(t, s)
	if reached := booksCalls(counter.take()); len(reached) > 0 {
		t.Errorf("refreshing without an export reached into books/: %v", reached)
	}
}

func TestRemoteShelfFallsBackOnUnusableExport(t *testing.T) {
	cases := map[string]func(cache *BookCacheFile){
		"escaping path": func(cache *BookCacheFile) {
			for id, entry := range cache.Books {
				entry.Path = "books/../outside.bookpkg"
				cache.Books[id] = entry
				return
			}
		},
		"mismatched id": func(cache *BookCacheFile) {
			for id, entry := range cache.Books {
				delete(cache.Books, id)
				cache.Books["someotherid0000"] = entry
				return
			}
		},
		"escaping folder": func(cache *BookCacheFile) {
			cache.Folders = append(cache.Folders, "../outside")
		},
	}

	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			libRoot := seedExportedShelf(t, "Dune", "Solaris")
			cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
			cache, err := readBookCacheFileAt(cachePath)
			if err != nil {
				t.Fatalf("read export: %v", err)
			}
			corrupt(cache)
			writeRawBookCache(t, libRoot, path.Base(cachePath), cache)

			s, counter := openRemoteShelf(t, libRoot, ShelfConf{})
			if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
				t.Errorf("listed %v, want %v", got, want)
			}
			if len(booksCalls(counter.take())) == 0 {
				t.Error("an unusable export was used instead of walking the shelf")
			}
		})
	}
}

func TestRemoteShelfPicksTheMostRecentWalk(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune", "Solaris")
	cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	current, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	// An older walk under a name that sorts after the current one and a newer
	// modification time: neither may win over the newer walk.
	older := *current
	older.WriterID = "zzolder"
	older.Timestamp = current.Timestamp - 100
	older.Books = map[string]BookCacheEntry{}
	for id, entry := range current.Books {
		if entry.Meta.Title == "Dune" {
			older.Books[id] = entry
		}
	}
	writeRawBookCache(t, libRoot, bookCacheFilePrefix+"zzolder"+bookCacheFileSuffix, older)

	s, _ := openRemoteShelf(t, libRoot, ShelfConf{})
	if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v from the most recent walk", got, want)
	}
}

func TestRemoteShelfIsReadOnly(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune")
	conf := ShelfConf{ReadOnly: false}

	s, _ := openRemoteShelf(t, libRoot, conf)
	if conf.ReadOnly {
		t.Error("NewShelfFromFS changed the caller's configuration")
	}
	if !s.ReadOnly() {
		t.Error("ReadOnly() = false, want true for a shelf opened from a ReadFS")
	}
	if _, err := s.NewBook(FolderPath{}, "New"); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("NewBook error = %v, want %v", err, fsutil.ErrReadOnly)
	}
	if err := s.NewFolder(FolderPath{}, "added"); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("NewFolder error = %v, want %v", err, fsutil.ErrReadOnly)
	}
}

func TestRemoteShelfPicksUpAReexportInTheSameSecond(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune", "Solaris")
	cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	first, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	s, _ := openRemoteShelf(t, libRoot, ShelfConf{ScanInterval: "1ms", BookCheckInterval: "1ms"})

	local := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if _, err := local.NewBook(FolderPath{}, "Hyperion"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := local.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cache, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.Timestamp = first.Timestamp
	writeRawBookCache(t, libRoot, path.Base(cachePath), cache)

	time.Sleep(5 * time.Millisecond)
	if got, want := listedTitles(t, s), []string{"Dune", "Hyperion", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("after a re-export in the same second, listed %v, want %v", got, want)
	}
}
