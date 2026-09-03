package shelf

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path"
	"testing"
	"time"
)

// writeBookOnDisk drops a book package into books/ the way an external file
// operation would: PlainShelf never sees the write, so only a walk can find it.
func writeBookOnDisk(t *testing.T, libRoot, dirName, bookID, title string) {
	t.Helper()

	bookDir := path.Join(libRoot, booksFolder, dirName+".bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": BookMetaSchemaVersion,
		"id":             bookID,
		"title":          title,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestRescanFindsBooksAddedOnDiskWithinTheScanInterval(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{
		LibRoot:  libRoot,
		LockMode: "none",
		// Long enough that nothing but the explicit rescan can walk the tree
		// again, which is the whole point of the endpoint behind it.
		ScanInterval: "24h",
	})

	writeBookOnDisk(t, libRoot, "added-behind-our-back", "onda1skb", "On Disk")

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 0 {
		t.Fatalf("ListBooks found %d books before the rescan, want 0", len(books))
	}

	result, err := s.Rescan()
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if result.BookCount != 1 {
		t.Errorf("BookCount = %d, want 1", result.BookCount)
	}
	if result.ID == "" {
		t.Error("Rescan returned an empty scan ID")
	}
	if result.StartedAt.IsZero() {
		t.Error("Rescan returned a zero start time")
	}

	books, err = s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks after rescan: %v", err)
	}
	if len(books) != 1 || books[0].ID() != "onda1skb" {
		t.Fatalf("ListBooks after rescan = %v, want the book added on disk", books)
	}
}

// The folder count describes the shelf, not the books, so a folder nobody has
// filed a book into still counts.
func TestRescanCountsFoldersIncludingEmptyOnes(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	if err := s.NewFolder(FolderPath{"Fiction"}, "Classics"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if err := s.NewFolder(FolderPath{}, "Empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if _, err := s.NewBook(FolderPath{"Fiction", "Classics"}, "Dune"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	result, err := s.Rescan()
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if result.BookCount != 1 {
		t.Errorf("BookCount = %d, want 1", result.BookCount)
	}
	// "/", "Empty", "Fiction", "Fiction/Classics".
	if result.FolderCount != 4 {
		t.Errorf("LayerCount = %d, want 4", result.FolderCount)
	}
}

// A rescan is a read. Unlike ExportBookCache, which forces the same walk, it
// must leave nothing behind on disk.
func TestRescanWritesNothing(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		ScanInterval:      "24h",
		// Long enough that the export cannot come due on its own during the test.
		BookCacheInterval: "24h",
	})
	waitForBookCacheExport(t, libRoot, testWriterID)

	before := readBookCacheFileOrFail(t, libRoot)

	writeBookOnDisk(t, libRoot, "added-behind-our-back", "onda1skb", "On Disk")
	if _, err := s.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	after := readBookCacheFileOrFail(t, libRoot)
	if _, ok := after.Books["onda1skb"]; ok {
		t.Error("Rescan exported the book cache; it must not write to the shelf")
	}
	if after.Timestamp != before.Timestamp {
		t.Errorf("exported cache timestamp changed from %d to %d; Rescan must not write",
			before.Timestamp, after.Timestamp)
	}
}

func readBookCacheFileOrFail(t *testing.T, libRoot string) BookCacheFile {
	t.Helper()

	filePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(filePath)
	if err != nil {
		t.Fatalf("read exported book cache: %v", err)
	}
	return *cache
}

func TestRescanRefusesASecondWalkAndNamesTheRunningOne(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	// Claimed directly rather than by racing two Rescan calls: the refusal is
	// what is under test, and a race that has to be won to observe it would be
	// flaky in exactly the direction that hides a regression.
	runningID := s.beginRescan(time.Now(), true).scanID
	if runningID == "" {
		t.Fatal("beginRescan did not claim a free shelf")
	}

	result, err := s.Rescan()
	if !errors.Is(err, ErrRescanInProgress) {
		t.Fatalf("Rescan error = %v, want ErrRescanInProgress", err)
	}
	if result.ID != runningID {
		t.Errorf("refused result ID = %q, want the running scan's ID %q", result.ID, runningID)
	}
	if result.BookCount != 0 || result.FolderCount != 0 {
		t.Error("a refused rescan reported counts it never walked for")
	}

	s.endRescan()
	if _, err := s.Rescan(); err != nil {
		t.Fatalf("Rescan after the running one ended: %v", err)
	}
}

// The loop the rate limit exists for: repeated calls that each cost a full walk,
// arriving far faster than a hand could produce them.
func TestRescanRefusesAConsecutiveLoopOnceTheBurstIsSpent(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	for i := range rescanBurst {
		if _, err := s.Rescan(); err != nil {
			t.Fatalf("Rescan %d of the burst: %v", i+1, err)
		}
	}

	result, err := s.Rescan()
	if !errors.Is(err, ErrRescanRateLimited) {
		t.Fatalf("Rescan past the burst: error = %v, want ErrRescanRateLimited", err)
	}
	if errors.Is(err, ErrRescanInProgress) {
		t.Error("a rate-limited rescan also matched ErrRescanInProgress; the two must stay distinguishable")
	}
	if result.RetryAfter <= 0 || result.RetryAfter > rescanRefill {
		t.Errorf("RetryAfter = %v, want a wait within one refill interval (%v)", result.RetryAfter, rescanRefill)
	}
	if result.BookCount != 0 || result.FolderCount != 0 {
		t.Error("a refused rescan reported counts it never walked for")
	}
}

// The reverse condition: pressing the button, letting the walk finish, and
// pressing it again is the sequence the button exists for and must never be
// refused. Run well past the burst to prove it is the pace, not the count, that
// the limit measures.
func TestRescanAllowsAHumanPacePressAfterPress(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	for i := range rescanBurst * 3 {
		if _, err := s.Rescan(); err != nil {
			t.Fatalf("Rescan %d at a human pace: %v", i+1, err)
		}

		// Stands in for the user reading the result before pressing again. The
		// clock is only advanced inside the limiter, so the test does not sleep.
		s.bookCache.Lock()
		s.bookCache.rescanTokensAt = s.bookCache.rescanTokensAt.Add(-rescanRefill)
		s.bookCache.Unlock()
	}
}

func TestRescanTokensRefillOverTime(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})
	now := time.Now()

	for i := range rescanBurst {
		claim := s.beginRescan(now, true)
		if claim.scanID == "" {
			t.Fatalf("claim %d of the burst was refused: %+v", i+1, claim)
		}
		s.endRescan()
	}

	claim := s.beginRescan(now, true)
	if claim.retryAfter <= 0 {
		t.Fatalf("claim past the burst was allowed: %+v", claim)
	}

	// One refill short, then one refill on.
	if claim := s.beginRescan(now.Add(rescanRefill/2), true); claim.retryAfter <= 0 {
		t.Errorf("half a refill later the claim was allowed: %+v", claim)
	}
	claim = s.beginRescan(now.Add(rescanRefill), true)
	if claim.scanID == "" {
		t.Errorf("a full refill later the claim was still refused: %+v", claim)
	}
	s.endRescan()

	// A long absence refills the bucket but never past the burst, so a shelf
	// left alone overnight does not hand out an unbounded run of walks.
	later := now.Add(24 * time.Hour)
	for i := range rescanBurst {
		if claim := s.beginRescan(later, true); claim.scanID == "" {
			t.Fatalf("claim %d after a long idle period was refused: %+v", i+1, claim)
		}
		s.endRescan()
	}
	if claim := s.beginRescan(later, true); claim.retryAfter <= 0 {
		t.Errorf("the bucket refilled past its burst: %+v", claim)
	}
}

// A rescan refused because another walk holds the shelf must not spend a token:
// otherwise a loop against a busy shelf drains the bucket for free and the next
// real user is told they are going too fast when they are not.
func TestRescanConflictDoesNotSpendARateLimitToken(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	runningID := s.beginRescan(time.Now(), true).scanID
	if runningID == "" {
		t.Fatal("beginRescan did not claim a free shelf")
	}
	for range rescanBurst * 20 {
		if _, err := s.Rescan(); !errors.Is(err, ErrRescanInProgress) {
			t.Fatalf("Rescan against a busy shelf: error = %v, want ErrRescanInProgress", err)
		}
	}
	s.endRescan()

	// The claim above spent one token; the rest of the burst must still be there.
	for i := range rescanBurst - 1 {
		if _, err := s.Rescan(); err != nil {
			t.Fatalf("Rescan %d after the conflicts: %v", i+1, err)
		}
	}
}

// The folder-transfer preflight forces the same walk from inside a larger
// operation. It must not spend the button's budget, and must not be refused
// once that budget is gone: it has nowhere to report the refusal and would
// plan from the stale cache it exists to avoid.
func TestRescanUnthrottledNeitherSpendsNorIsRefusedByTheRateLimit(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	// Stands in for a run of transfers, each forcing its own walk.
	for i := range rescanBurst * 4 {
		if _, err := s.RescanUnthrottled(); err != nil {
			t.Fatalf("RescanUnthrottled %d: %v", i+1, err)
		}
	}

	// The button's budget is untouched, so a user who never pressed it does not
	// find it already spent.
	for i := range rescanBurst {
		if _, err := s.Rescan(); err != nil {
			t.Fatalf("Rescan %d after the transfers: %v", i+1, err)
		}
	}

	// And with the budget now genuinely spent, the preflight still walks.
	if _, err := s.Rescan(); !errors.Is(err, ErrRescanRateLimited) {
		t.Fatalf("Rescan past the burst: error = %v, want ErrRescanRateLimited", err)
	}
	if _, err := s.RescanUnthrottled(); err != nil {
		t.Fatalf("RescanUnthrottled with the bucket empty: %v", err)
	}

	// The singleflight still applies to it: it is the rate limit it skips, not
	// the refusal to run two walks over the same shelf at once. Claimed without
	// the limit because the bucket is empty by this point.
	runningID := s.beginRescan(time.Now(), false).scanID
	if runningID == "" {
		t.Fatal("beginRescan did not claim a free shelf")
	}
	defer s.endRescan()
	if _, err := s.RescanUnthrottled(); !errors.Is(err, ErrRescanInProgress) {
		t.Fatalf("RescanUnthrottled against a busy shelf: error = %v, want ErrRescanInProgress", err)
	}
}
