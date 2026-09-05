package shelf

import (
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

// ErrRescanInProgress reports that a manual rescan is already walking this
// shelf, so this one was refused rather than started alongside it.
//
// Refused rather than queued or attached: a second walk would cost the same
// directory traversal to answer a question the first is already answering, and
// on the SMB and cloud-mounted shelves this button exists for, that traversal
// is the expensive part. Attaching to the running walk is worse for the very
// reason the button exists — a walk that began before the user dropped their
// file in cannot report it, so attaching would answer "scanned, still not
// there" while a retry after it ends finds the book.
var ErrRescanInProgress = util.NewError("a rescan is already in progress")

// ErrRescanRateLimited reports that this caller has asked for full walks faster
// than the shelf is willing to perform them, so this one was refused.
//
// Separate from ErrRescanInProgress because they ask the caller for different
// things: a walk in progress ends on its own and the answer is to wait for it,
// while a rate limit is a statement about the caller's own pace. A client that
// could not tell them apart would retry the second as if it were the first.
var ErrRescanRateLimited = util.NewError("rescans are being requested too quickly")

// The manual rescan's rate limit, as a token bucket. It is not configurable:
// both values are chosen to sit far above any human pace rather than to suit a
// particular shelf, and shelf.json is a compatibility-sensitive format that a
// knob nobody can usefully tune should not grow.
//
// A fixed minimum interval was rejected. Measured from either end of the
// previous walk it refuses a user who presses the button, waits for the walk,
// and presses again — the one sequence this button exists for. A bucket splits
// the two demands that pull against each other into separate numbers: the burst
// is what a hand can spend, the refill is what a loop is left with.
const (
	// rescanBurst is how many walks can be started back to back.
	rescanBurst = 5

	// rescanRefill is how long one token takes to come back, so the sustained
	// ceiling is six walks a minute. On an SMB shelf the singleflight already
	// holds the rate to one walk per walk; this is what bounds a fast local
	// shelf, where an unthrottled loop can start walks as fast as they finish.
	rescanRefill = 10 * time.Second
)

// RescanResult reports what a manual rescan found.
type RescanResult struct {
	// ID names this rescan while it runs. It is also filled in on the
	// ErrRescanInProgress failure, where it names the rescan that was already
	// running, so a caller can tell its own user which one to wait for.
	ID string

	// StartedAt is when the walk began, not when it finished; see
	// scanToBookCache for why the shelf's freshness is dated from the start.
	StartedAt time.Time

	BookCount   int
	FolderCount int

	// RetryAfter is filled in only on the ErrRescanRateLimited failure, where it
	// is how long the caller must wait before a walk would be allowed. It is the
	// rate limit's counterpart to ID on ErrRescanInProgress: the one thing the
	// refused caller needs in order to tell its user what to do next.
	RetryAfter time.Duration
}

// Rescan walks the shelf now and rebuilds the book cache from what it finds.
//
// scanInterval does not apply: it exists to stop repeated browsing from
// re-walking the tree, and a user who pressed a button has already answered the
// question it asks. This is the one unconditional full scan the API can reach
// without a side effect — ExportBookCache also forces a walk but writes a file
// afterwards, the wrong thing to reach for when all that is wanted is for a book
// dropped into books/ to show up.
func (s *Shelf) Rescan() (RescanResult, error) {
	return s.rescan(true)
}

// RescanUnthrottled is Rescan without the rate limit: it neither spends a token
// nor is refused for want of one.
//
// It is for a forced walk performed *inside* a larger operation, not for one a
// request asked for by itself — today, the folder-transfer preflight, which
// walks both shelves so the plan and the conflict checks read an authoritative
// listing. Two things make the rate limit wrong there. The tokens belong to the
// button: spending them on a transfer would let a user who moved five folders
// find that "update book list" now answers 429 for something they never
// pressed. And the caller has nowhere to put the refusal — it would silently
// plan from a stale cache, which is the one thing that preflight exists to
// prevent.
//
// This is not a judgement that the transfer route needs no bound on forced
// walks. It has none today and had none before the rate limit existed; bounding
// every path that forces a walk — this one and ExportBookCache — is its own
// piece of work, and one that has to answer for the whole operation's cost
// rather than for the walk alone.
func (s *Shelf) RescanUnthrottled() (RescanResult, error) {
	return s.rescan(false)
}

func (s *Shelf) rescan(rateLimited bool) (RescanResult, error) {
	if !s.IsReady() {
		return RescanResult{}, util.Errorf("%w", ErrShelfInitializing)
	}

	claim := s.beginRescan(time.Now(), rateLimited)
	switch {
	case claim.runningID != "":
		return RescanResult{ID: claim.runningID}, util.Errorf("%w", ErrRescanInProgress)
	case claim.retryAfter > 0:
		return RescanResult{RetryAfter: claim.retryAfter}, util.Errorf("%w", ErrRescanRateLimited)
	}
	defer s.endRescan()

	// The shared lock is the one every read takes, so holding it for the walk
	// leaves the library readable throughout. It excludes only the structural
	// writes that would move a book out from under the walk.
	if err := s.shelfLock.RLock(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	if err := s.scanToBookCache(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}

	// Both counts come from the cache the walk just filled, which is also why
	// the folder count includes a folder holding no books: the walk reports every
	// folder directory it enters, and a folder the user made but has not filed
	// anything into is still part of the shelf this number describes.
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	return RescanResult{
		ID:          claim.scanID,
		StartedAt:   s.bookCache.lastScanStart,
		BookCount:   len(s.bookCache.cache),
		FolderCount: len(s.bookCache.folders),
	}, nil
}

// rescanClaim is the outcome of trying to claim the shelf for one rescan.
// Exactly one field is set: scanID on success, runningID when another walk
// holds the shelf, retryAfter when the rate limit refused this one.
type rescanClaim struct {
	scanID     string
	runningID  string
	retryAfter time.Duration
}

// beginRescan claims the shelf for one rescan, taking now as the clock so a
// test can move the rate limit without sleeping. With rateLimited false it
// claims the shelf without touching the bucket at all; see RescanUnthrottled.
//
// The running walk is checked before the token: a caller refused with 409 never
// pays for a walk it did not get, so a loop hammering a busy shelf cannot drain
// the bucket and leave the next real user with a 429 that misdescribes what
// happened.
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

	// Not a cryptographic identifier: it is never persisted and never
	// authenticates anything. It exists so a refused caller can say which
	// rescan it lost to, and so two of them can be told apart in the log.
	s.bookCache.rescanID = shelfutil.RandomString(12)
	return rescanClaim{scanID: s.bookCache.rescanID}
}

// takeRescanToken spends one token and reports 0, or reports how long until the
// next one is available and spends nothing. It must be called with the book
// cache locked.
func (c *bookCache) takeRescanToken(now time.Time) time.Duration {
	// Clamped at zero: a clock that stepped backwards, which on a laptop
	// resuming from sleep is ordinary, must not hand out tokens by refilling a
	// negative interval, nor deny them by carrying a negative debt forward.
	if elapsed := now.Sub(c.rescanTokensAt); elapsed > 0 {
		c.rescanTokens = min(rescanBurst, c.rescanTokens+float64(elapsed)/float64(rescanRefill))
	}
	c.rescanTokensAt = now

	if c.rescanTokens < 1 {
		// Exact, not rounded: the caller decides how to present it, and the HTTP
		// layer is the only one that has to flatten this to whole seconds.
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
