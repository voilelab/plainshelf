package logutil

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// seedLogFile writes a file into dir, whether or not it is one the writer is
// expected to recognize as a log.
func seedLogFile(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("entry\n"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
	return path
}

// remainingNames lists what survived a rotation, sorted for comparison.
func remainingNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func TestDailyFileWriterRotateRemovesExpired(t *testing.T) {
	dir := t.TempDir()
	seedLogFile(t, dir, "app-2024-01-01.log")
	seedLogFile(t, dir, "app-2024-01-09.log")
	seedLogFile(t, dir, "app-2024-01-10.log")

	w := newTestWriter(dir, "app", 3)
	t.Cleanup(func() { _ = w.Close() })

	if err := w.rotate("2024-01-12"); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Retention is an age, so 01-09 is exactly three days old and stays while
	// 01-01 is well past the window.
	want := []string{"app-2024-01-09.log", "app-2024-01-10.log", "app-2024-01-12.log"}
	if got := remainingNames(t, dir); !equalNames(got, want) {
		t.Fatalf("remaining = %v, want %v", got, want)
	}
}

func TestDailyFileWriterRotateKeepsEverythingWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	seedLogFile(t, dir, "app-2020-01-01.log")

	w := newTestWriter(dir, "app", 0)
	t.Cleanup(func() { _ = w.Close() })

	if err := w.rotate("2024-01-12"); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	want := []string{"app-2020-01-01.log", "app-2024-01-12.log"}
	if got := remainingNames(t, dir); !equalNames(got, want) {
		t.Fatalf("remaining = %v, want %v", got, want)
	}
}

// TestDailyFileWriterRotateOnlyRemovesLogFiles pins the acceptance rule that
// cleanup deletes exactly what the log API lists: the same prefix, date stamp
// and suffix, and never a directory or a symlink.
func TestDailyFileWriterRotateOnlyRemovesLogFiles(t *testing.T) {
	dir := t.TempDir()

	target := seedLogFile(t, dir, "keep-me.txt")
	seedLogFile(t, dir, "notes.txt")
	seedLogFile(t, dir, "other-2020-01-01.log")
	seedLogFile(t, dir, "app-2020-01-01.txt")
	seedLogFile(t, dir, "app-yesterday.log")
	seedLogFile(t, dir, "app-2020-01-01.log")

	if err := os.Mkdir(filepath.Join(dir, "app-2020-01-02.log"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "app-2020-01-03.log")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	w := newTestWriter(dir, "app", 30)
	t.Cleanup(func() { _ = w.Close() })

	if err := w.rotate("2024-01-12"); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Only app-2020-01-01.log matches the listing rules and is expired.
	want := []string{
		"app-2020-01-01.txt",
		"app-2020-01-02.log",
		"app-2020-01-03.log",
		"app-2024-01-12.log",
		"app-yesterday.log",
		"keep-me.txt",
		"notes.txt",
		"other-2020-01-01.log",
	}
	if got := remainingNames(t, dir); !equalNames(got, want) {
		t.Fatalf("remaining = %v, want %v", got, want)
	}

	// The symlink is skipped, not followed: its target has to survive too.
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat symlink target: %v", err)
	}
}

// TestDailyFileWriterRotateExpiresTheClosedFile covers a rotation that skips
// days: the file just closed is already out of the window, and the one now
// being written to is not.
func TestDailyFileWriterRotateExpiresTheClosedFile(t *testing.T) {
	dir := t.TempDir()

	w := newTestWriter(dir, "app", 1)
	t.Cleanup(func() { _ = w.Close() })

	if err := w.rotate("2024-01-12"); err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	if err := w.rotate("2024-01-14"); err != nil {
		t.Fatalf("second rotate: %v", err)
	}

	want := []string{"app-2024-01-14.log"}
	if got := remainingNames(t, dir); !equalNames(got, want) {
		t.Fatalf("remaining = %v, want %v", got, want)
	}
}

// newTestWriter builds a rotating writer over dir with a fixed window.
func newTestWriter(dir, prefix string, retentionDays int) *DailyFileWriter {
	return NewDailyFileWriter(LogFileConf{
		Type:          LogFileTypeNameRotate,
		Dir:           dir,
		Prefix:        prefix,
		RetentionDays: &retentionDays,
	})
}

// TestDailyFileWriterRotateUsesTheRuntimeWindow covers the setting route
// changing retention while the server runs: the writer reads the shared value
// on its next rotation rather than the one it was built with.
func TestDailyFileWriterRotateUsesTheRuntimeWindow(t *testing.T) {
	dir := t.TempDir()
	seedLogFile(t, dir, "app-2024-01-01.log")

	retention := NewRetention()
	days := 3
	w := NewDailyFileWriter(LogFileConf{
		Type:          LogFileTypeNameRotate,
		Dir:           dir,
		Prefix:        "app",
		RetentionDays: &days,
		Retention:     retention,
	})
	t.Cleanup(func() { _ = w.Close() })

	// Set to keep everything: the configured three-day window must not apply.
	retention.Set(0)
	if err := w.rotate("2024-01-12"); err != nil {
		t.Fatalf("rotate under an override: %v", err)
	}
	if got := remainingNames(t, dir); !equalNames(got, []string{"app-2024-01-01.log", "app-2024-01-12.log"}) {
		t.Fatalf("remaining under an override = %v, want the expired file kept", got)
	}

	// Clearing it returns the writer to its configured window.
	retention.Clear()
	if err := w.rotate("2024-01-13"); err != nil {
		t.Fatalf("rotate after clearing: %v", err)
	}
	// The three-day window now applies again: 01-01 is expired, 01-12 is not.
	if got := remainingNames(t, dir); !equalNames(got, []string{"app-2024-01-12.log", "app-2024-01-13.log"}) {
		t.Fatalf("remaining after clearing = %v, want the expired file gone", got)
	}
}

func TestRetentionDaysFallsBackToTheConfiguredWindow(t *testing.T) {
	var unset *Retention
	if got := unset.Days(7); got != 7 {
		t.Fatalf("nil Retention Days(7) = %d, want 7", got)
	}

	retention := NewRetention()
	if got := retention.Days(7); got != 7 {
		t.Fatalf("unset Retention Days(7) = %d, want 7", got)
	}

	retention.Set(2)
	if got := retention.Days(7); got != 2 {
		t.Fatalf("Days(7) after Set(2) = %d, want 2", got)
	}

	retention.Set(-1)
	if got := retention.Days(7); got != 0 {
		t.Fatalf("Days(7) after Set(-1) = %d, want 0", got)
	}

	retention.Clear()
	if got := retention.Days(7); got != 7 {
		t.Fatalf("Days(7) after Clear = %d, want 7", got)
	}
}

func TestNewLogFileRejectsNegativeRetention(t *testing.T) {
	days := -1
	if _, err := NewLogFile(LogFileConf{
		Type:          LogFileTypeNameRotate,
		Dir:           t.TempDir(),
		Prefix:        "app",
		RetentionDays: &days,
	}); err == nil {
		t.Fatal("NewLogFile accepted a negative retention window")
	}
}

func TestLogFileConfRetentionDaysDefaults(t *testing.T) {
	if got := (LogFileConf{}).ResolvedRetentionDays(); got != DefaultRetentionDays {
		t.Fatalf("unset retentionDays = %d, want %d", got, DefaultRetentionDays)
	}

	zero := 0
	if got := (LogFileConf{RetentionDays: &zero}).ResolvedRetentionDays(); got != 0 {
		t.Fatalf("explicit zero retentionDays = %d, want 0", got)
	}
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
