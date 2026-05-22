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

	curDate string
	file    *os.File
}

func NewDailyFileWriter(dir, prefix string) *DailyFileWriter {
	if prefix == "" {
		prefix = "log"
	}

	return &DailyFileWriter{
		dir:    dir,
		prefix: prefix,
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

	return nil
}

func (w *DailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	return w.file.Close()
}
