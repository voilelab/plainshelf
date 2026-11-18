//go:build !(js && wasm)

package store

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/util"
)

func lockStore(dir string) (func() error, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, util.Errorf("%w", err)
	}
	lk := flock.New(filepath.Join(dir, settingsFile+".lock"))
	locked, err := lk.TryLock()
	if err != nil {
		return nil, util.Errorf("locking settings store: %w", err)
	}
	if !locked {
		return nil, util.Errorf("settings store is already open: %s", dir)
	}
	return lk.Unlock, nil
}
