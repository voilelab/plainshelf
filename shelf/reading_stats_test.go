package shelf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func newTestReadingStats(t *testing.T) (*ReadingStats, string) {
	t.Helper()
	root := t.TempDir()
	return newReadingStats(fsutil.NewLocalFS(root)), root
}

func TestReadingStatsAddAndFlushRoundTrip(t *testing.T) {
	rs, root := newTestReadingStats(t)

	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 60); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}
	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 40); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}
	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-b", 30); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}

	if err := rs.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	filePath := filepath.Join(root, "app", "stats", "reading", "2026-07.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected stats file to exist at %s: %v", filePath, err)
	}

	// Load into a fresh ReadingStats to confirm the file round-trips.
	reloaded := newReadingStats(fsutil.NewLocalFS(root))
	days, err := reloaded.Range(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	day, ok := days["2026-07-13"]
	if !ok {
		t.Fatalf("expected day 2026-07-13 in range result, got %#v", days)
	}
	if day.TotalSeconds != 130 {
		t.Fatalf("total_seconds = %d, want 130", day.TotalSeconds)
	}
}

func TestReadingStatsCrossMonthSplitsFiles(t *testing.T) {
	rs, root := newTestReadingStats(t)

	if err := rs.AddSeconds(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), "book-a", 50); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}
	if err := rs.AddSeconds(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), "book-a", 70); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}

	if err := rs.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	for _, month := range []string{"2026-06", "2026-07"} {
		filePath := filepath.Join(root, "app", "stats", "reading", month+".json")
		if _, err := os.Stat(filePath); err != nil {
			t.Fatalf("expected month file %s to exist: %v", filePath, err)
		}
	}

	days, err := rs.Range(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if days["2026-06-30"].TotalSeconds != 50 {
		t.Fatalf("2026-06-30 total_seconds = %d, want 50", days["2026-06-30"].TotalSeconds)
	}
	if days["2026-07-01"].TotalSeconds != 70 {
		t.Fatalf("2026-07-01 total_seconds = %d, want 70", days["2026-07-01"].TotalSeconds)
	}
}

func TestReadingStatsClampsPerCallSeconds(t *testing.T) {
	rs, _ := newTestReadingStats(t)

	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 9999); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}
	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", -50); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}

	days, err := rs.Range(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	// First call clamped to 120 (maxSecondsPerCall), second call clamped to 0.
	if days["2026-07-13"].TotalSeconds != 120 {
		t.Fatalf("total_seconds = %d, want 120 (single call clamp)", days["2026-07-13"].TotalSeconds)
	}
}

func TestReadingStatsClampsDailyTotal(t *testing.T) {
	rs, _ := newTestReadingStats(t)

	// 24h = 86400s. Add far more than that across many calls; per-call clamp is
	// 120s so we need many calls, but the daily total clamp should still cap
	// the sum well short of naive addition.
	for i := range 800 {
		if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 120); err != nil {
			t.Fatalf("AddSeconds call %d: %v", i, err)
		}
	}
	// 800 * 120 = 96000, which exceeds maxSecondsPerDay (86400).

	days, err := rs.Range(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if days["2026-07-13"].TotalSeconds != maxSecondsPerDay {
		t.Fatalf("total_seconds = %d, want %d (daily clamp)", days["2026-07-13"].TotalSeconds, maxSecondsPerDay)
	}
}

func TestReadingStatsEmptyBookID(t *testing.T) {
	rs, _ := newTestReadingStats(t)

	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "", 10); err == nil {
		t.Fatalf("expected error for empty book id, got nil")
	}
}

func TestReadingStatsMissingMonthFileReturnsEmpty(t *testing.T) {
	rs, _ := newTestReadingStats(t)

	// No AddSeconds/Flush ever happened - nothing on disk. This simulates an
	// old vault created before reading stats existed.
	days, err := rs.Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range on missing files should not error, got: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("expected empty days map for a vault with no stats file, got %#v", days)
	}
}

func TestReadingStatsFlushIsNoopWithoutDirtyData(t *testing.T) {
	rs, root := newTestReadingStats(t)

	if err := rs.Flush(); err != nil {
		t.Fatalf("Flush with no dirty data: %v", err)
	}

	statsDir := filepath.Join(root, "app", "stats", "reading")
	if _, err := os.Stat(statsDir); err == nil {
		t.Fatalf("expected no stats directory to be created when nothing is dirty")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking stats dir: %v", err)
	}
}

func TestReadingStatsRangeRejectsInvertedRange(t *testing.T) {
	rs, _ := newTestReadingStats(t)

	if _, err := rs.Range(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatalf("expected error for inverted range, got nil")
	}
}

// hookFS wraps an FS and runs onWriteFile before delegating WriteFile calls.
type hookFS struct {
	fsutil.FS
	onWriteFile func()
}

func (h *hookFS) WriteFile(name string, data []byte) error {
	if h.onWriteFile != nil {
		h.onWriteFile()
	}
	return h.FS.WriteFile(name, data)
}

func TestReadingStatsFlushKeepsMonthDirtyWhenMutatedDuringWrite(t *testing.T) {
	root := t.TempDir()
	hooked := &hookFS{FS: fsutil.NewLocalFS(root)}
	rs := newReadingStats(hooked)

	if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 60); err != nil {
		t.Fatalf("AddSeconds: %v", err)
	}

	// Simulate a heartbeat landing while Flush is writing the snapshot to disk
	// (i.e. after Flush released the lock but before it clears dirty marks).
	hooked.onWriteFile = func() {
		hooked.onWriteFile = nil // fire once; later flushes write normally
		if err := rs.AddSeconds(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), "book-a", 30); err != nil {
			t.Errorf("AddSeconds during flush: %v", err)
		}
	}

	if err := rs.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// The mid-write heartbeat is not part of the file just written, so the
	// month must have stayed dirty and this second Flush must persist it.
	if err := rs.Flush(); err != nil {
		t.Fatalf("second Flush: %v", err)
	}

	reloaded := newReadingStats(fsutil.NewLocalFS(root))
	days, err := reloaded.Range(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	day, ok := days["2026-07-13"]
	if !ok {
		t.Fatalf("expected day 2026-07-13 in range result, got %#v", days)
	}
	if day.TotalSeconds != 90 {
		t.Fatalf("total_seconds = %d, want 90 (heartbeat recorded during flush was lost)", day.TotalSeconds)
	}
}
