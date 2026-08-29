package logutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DailyFileWriter struct {
	mu     sync.Mutex
	dir    string
	prefix string

	// retentionDays is the age past which a rotated file is removed, and
	// retention is the runtime value that overrides it when one is shared with
	// this writer. Zero keeps every file.
	retentionDays int
	retention     *Retention

	curDate string
	file    *os.File
}

func NewDailyFileWriter(conf LogFileConf) *DailyFileWriter {
	prefix := conf.Prefix
	if prefix == "" {
		prefix = "log"
	}

	retentionDays := conf.ResolvedRetentionDays()
	if retentionDays < 0 {
		retentionDays = 0
	}

	return &DailyFileWriter{
		dir:           conf.Dir,
		prefix:        prefix,
		retentionDays: retentionDays,
		retention:     conf.Retention,
	}
}

func (w *DailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if today != w.curDate {
		if err := w.rotate(today); err != nil {
			return 0, err
		}
	}

	return w.file.Write(p)
}

func (w *DailyFileWriter) rotate(date string) error {
	if err := os.MkdirAll(w.dir, 0755); err != nil {
		return err
	}

	name := fmt.Sprintf("%s-%s.log", w.prefix, date)
	path := filepath.Join(w.dir, name)

	newFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	oldFile := w.file
	w.file = newFile
	w.curDate = date

	if oldFile != nil {
		_ = oldFile.Close()
	}

	// Expiring on rotation rather than on a timer keeps disk activity at zero
	// while nobody is using the server, and the first Write after a restart
	// rotates, so a long-idle instance still cleans up when it wakes.
	w.removeExpired(date)

	return nil
}

// removeExpired deletes rotated files older than the retention window.
//
// It is best-effort and reports nothing: this writer is the sink the logger
// itself writes to, so a failure here has nowhere to be logged. A file that
// cannot be removed is simply retried on the next rotation.
func (w *DailyFileWriter) removeExpired(date string) {
	// Read while holding the writer lock, which is why Retention holds a value
	// rather than calling back: anything that logged here would re-enter Write.
	retentionDays := w.retention.Days(w.retentionDays)
	if retentionDays <= 0 {
		return
	}

	today, err := time.Parse(logDateLayout, date)
	if err != nil {
		return
	}
	cutoff := today.AddDate(0, 0, -retentionDays)

	// Listing through listRotatedLogFiles is deliberate: deletion has to agree
	// with what the log API calls a log file, or a hand-rolled match here could
	// delete something the server never listed.
	entries, err := listRotatedLogFiles("", LogFileConf{
		Type:   LogFileTypeNameRotate,
		Dir:    w.dir,
		Prefix: w.prefix,
	})
	if err != nil {
		return
	}

	current := ""
	if w.file != nil {
		current = cleanLogPath(w.file.Name())
	}

	for _, entry := range entries {
		if entry.path == current {
			continue
		}
		entryDate, err := time.Parse(logDateLayout, entry.Date)
		if err != nil || !entryDate.Before(cutoff) {
			continue
		}
		_ = os.Remove(entry.path)
	}
}

func (w *DailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	return w.file.Close()
}
