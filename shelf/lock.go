package shelf

import (
	"context"
	"time"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/util"
)

type ShelfLock interface {
	Lock() error
	RLock() error
	Unlock() error
	Close() error
}

type writeLockedShelf struct {
	*Shelf
}

func lockShelfForWrite(s *Shelf) (writeLockedShelf, error) {
	if err := s.shelfLock.Lock(); err != nil {
		return writeLockedShelf{}, util.Errorf("%w", err)
	}
	return writeLockedShelf{s}, nil
}

func lockForTransfer(source, target *Shelf) (writeLockedShelf, func(), error) {
	if err := source.shelfLock.RLock(); err != nil {
		return writeLockedShelf{}, nil, util.Errorf("%w", err)
	}
	locked, err := lockShelfForWrite(target)
	if err != nil {
		_ = source.shelfLock.Unlock()
		return writeLockedShelf{}, nil, util.Errorf("%w", err)
	}
	return locked, func() {
		_ = target.shelfLock.Unlock()
		_ = source.shelfLock.Unlock()
	}, nil
}

type flockLock struct {
	fl      *flock.Flock
	timeout time.Duration
}

func newFlockLock(lockFilePath string, timeout time.Duration) ShelfLock {
	return &flockLock{
		fl:      flock.New(lockFilePath),
		timeout: timeout,
	}
}

func (l *flockLock) Lock() error {
	if l.timeout == 0 {
		return l.fl.Lock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	locked, err := l.fl.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return util.Errorf("%w: %w", ErrShelfLockTimeout, err)
	}
	if !locked {
		return util.Errorf("%w: write lock timed out, another instance may hold it", ErrShelfLockTimeout)
	}
	return nil
}

func (l *flockLock) RLock() error {
	if l.timeout == 0 {
		return l.fl.RLock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	locked, err := l.fl.TryRLockContext(ctx, lockRetryDelay)
	if err != nil {
		return util.Errorf("%w: %w", ErrShelfLockTimeout, err)
	}
	if !locked {
		return util.Errorf("%w: read lock timed out, another instance may hold it", ErrShelfLockTimeout)
	}
	return nil
}

func (l *flockLock) Unlock() error {
	return l.fl.Unlock()
}

func (l *flockLock) Close() error {
	return l.fl.Close()
}

type noneLock struct{}

func newNoneLock() ShelfLock { return noneLock{} }

func (noneLock) Lock() error   { return nil }
func (noneLock) RLock() error  { return nil }
func (noneLock) Unlock() error { return nil }
func (noneLock) Close() error  { return nil }
