package shelf

import (
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

var ErrRescanInProgress = util.NewError("a rescan is already in progress")

var ErrRescanRateLimited = util.NewError("rescans are being requested too quickly")

const (
	rescanBurst = 5

	rescanRefill = 10 * time.Second
)

type RescanResult struct {
	ID string

	StartedAt time.Time

	BookCount   int
	FolderCount int

	RetryAfter time.Duration
}

func (s *Shelf) Rescan() (RescanResult, error) {
	return s.rescan(true)
}

func (s *Shelf) RescanUnthrottled() (RescanResult, error) {
	return s.rescan(false)
}

func (s *Shelf) rescan(rateLimited bool) (RescanResult, error) {
	if !s.IsReady() && s.awaitingSource.Load() {
		s.initRetryMu.Lock()
		var err error
		if !s.IsReady() {
			err = s.retryInit()
		}
		s.initRetryMu.Unlock()
		if err != nil {
			return RescanResult{}, err
		}
	}
	if !s.IsReady() {
		return RescanResult{}, s.notReady()
	}

	claim := s.beginRescan(time.Now(), rateLimited)
	switch {
	case claim.runningID != "":
		return RescanResult{ID: claim.runningID}, util.Errorf("%w", ErrRescanInProgress)
	case claim.retryAfter > 0:
		return RescanResult{RetryAfter: claim.retryAfter}, util.Errorf("%w", ErrRescanRateLimited)
	}
	defer s.endRescan()

	if err := s.shelfLock.RLock(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	if err := s.scanToBookCache(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	return RescanResult{
		ID:          claim.scanID,
		StartedAt:   s.bookCache.lastScanStart,
		BookCount:   len(s.bookCache.cache),
		FolderCount: len(s.bookCache.folders),
	}, nil
}

type rescanClaim struct {
	scanID     string
	runningID  string
	retryAfter time.Duration
}

func (s *Shelf) beginRescan(now time.Time, rateLimited bool) rescanClaim {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	if s.bookCache.rescanID != "" {
		return rescanClaim{runningID: s.bookCache.rescanID}
	}

	if rateLimited {
		if retryAfter := s.bookCache.takeRescanToken(now); retryAfter > 0 {
			return rescanClaim{retryAfter: retryAfter}
		}
	}

	s.bookCache.rescanID = fsutil.RandomString(12)
	return rescanClaim{scanID: s.bookCache.rescanID}
}

func (c *bookCache) takeRescanToken(now time.Time) time.Duration {
	if elapsed := now.Sub(c.rescanTokensAt); elapsed > 0 {
		c.rescanTokens = min(rescanBurst, c.rescanTokens+float64(elapsed)/float64(rescanRefill))
	}
	c.rescanTokensAt = now

	if c.rescanTokens < 1 {
		return time.Duration((1 - c.rescanTokens) * float64(rescanRefill))
	}

	c.rescanTokens--
	return 0
}

func (s *Shelf) endRescan() {
	s.bookCache.Lock()
	s.bookCache.rescanID = ""
	s.bookCache.Unlock()
}
