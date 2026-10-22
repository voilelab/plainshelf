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

	configuredRetentionDays int
	runtimeRetention        *Retention

	curDate string
	file    *os.File
}

func NewDailyFileWriter(conf LogFileConf) *DailyFileWriter {
	prefix := conf.Prefix
	if prefix == "" {
		prefix = "log"
	}

	retentionDays := max(conf.ResolvedRetentionDays(), 0)

	return &DailyFileWriter{
		dir:                     conf.Dir,
		prefix:                  prefix,
		configuredRetentionDays: retentionDays,
		runtimeRetention:        conf.Retention,
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

	w.removeExpiredLocked(date)

	return nil
}

func (w *DailyFileWriter) removeExpiredLocked(date string) {
	retentionDays := w.runtimeRetention.Days(w.configuredRetentionDays)
	if retentionDays <= 0 {
		return
	}

	today, err := time.Parse(logDateLayout, date)
	if err != nil {
		return
	}
	cutoff := today.AddDate(0, 0, -retentionDays)

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
