package lockedfile

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const maxDocumentBytes = 1 << 20

const (
	lockTimeout    = 10 * time.Second
	lockRetryDelay = 50 * time.Millisecond
)

type File struct {
	path        string
	lockPath    string
	label       string
	tempPattern string
}

// New takes the lock on a sibling ".lock" file rather than the document itself:
// the document is replaced by rename on every write, which would drop a lock
// held on the old inode.
func New(path, label, tempPattern string) *File {
	return &File{path: path, lockPath: path + ".lock", label: label, tempPattern: tempPattern}
}

func (f *File) Read() (string, error) {
	if f.path == "" {
		return "", f.notReady()
	}
	return fsutil.ReadTextFile(f.path)
}

func Mutate[T any](f *File, parse func(string) T, serialize func(T) (string, error), fn func(T) T) (T, error) {
	var zero T
	if f.path == "" {
		return zero, f.notReady()
	}

	unlock, err := f.lock()
	if err != nil {
		return zero, err
	}
	defer unlock()

	raw, err := fsutil.ReadTextFile(f.path)
	if err != nil {
		return zero, err
	}
	current := parse(raw)
	next := fn(current)

	currentText, err := serialize(current)
	if err != nil {
		return zero, err
	}
	nextText, err := serialize(next)
	if err != nil {
		return zero, err
	}
	if nextText == currentText {
		return next, nil
	}
	if len(nextText) > maxDocumentBytes {
		return zero, util.Errorf("%s document is too large: %d bytes", f.label, len(nextText))
	}

	if err := fsutil.WriteTextFileAtomic(f.path, f.tempPattern, nextText); err != nil {
		return zero, err
	}
	return next, nil
}

func (f *File) notReady() error {
	return util.Errorf("%s storage is not ready", f.label)
}

func (f *File) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(f.lockPath), 0o755); err != nil {
		return nil, util.Errorf("%w", err)
	}

	lk := flock.New(f.lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := lk.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return nil, util.Errorf("locking %s: %w", f.label, err)
	}
	if !locked {
		return nil, util.Errorf("timed out locking %s", f.label)
	}
	return func() { _ = lk.Unlock() }, nil
}
