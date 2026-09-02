package taskutil

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

const defaultMaxKeep = 100

var ErrTaskChainRunning = util.NewError("task chain is already running")

// CancelResult reports what Cancel did, so a caller can answer a cancel request
// precisely rather than guessing from the chain's status alone.
type CancelResult int

const (
	// CancelSignalled means a non-terminal chain was signalled to stop.
	CancelSignalled CancelResult = iota
	// CancelNotFound means no chain with the given ID is retained.
	CancelNotFound
	// CancelAlreadyTerminal means the chain exists but had already settled, so the
	// cancel was a no-op.
	CancelAlreadyTerminal
)

// Pool submits task chains to a worker and keeps them addressable by ID so that
// callers can poll their status after submission.
//
// The pool owns the worker: Start and Close delegate to it.
type Pool struct {
	worker  *Worker
	maxKeep int

	mu sync.RWMutex
	// byID holds every retained chain; order lists their IDs oldest first.
	byID  map[string]*TaskChain
	order []string
}

// NewPool returns a pool backed by w, retaining at most maxKeep chains.
func NewPool(w *Worker, maxKeep int) *Pool {
	if maxKeep <= 0 {
		maxKeep = defaultMaxKeep
	}

	return &Pool{
		worker:  w,
		maxKeep: maxKeep,
		byID:    map[string]*TaskChain{},
	}
}

func (p *Pool) Start() { p.worker.Start() }

func (p *Pool) Close() error { return p.worker.Close() }

// Submit registers the chain, assigns it an ID, and enqueues it.
//
// When the chain carries a Key that matches an active chain, the existing chain
// is returned together with ErrTaskChainRunning so that the caller can point the
// client at the work already in flight. Otherwise the submitted chain is
// returned.
func (p *Pool) Submit(chain *TaskChain) (*TaskChain, error) {
	if chain == nil {
		return nil, util.Errorf("task chain cannot be nil")
	}

	// The lock is held across the enqueue so that the duplicate-key check and
	// the registration that follows it cannot interleave with another Submit.
	// Worker.Run never acquires the pool lock, and it enqueues without
	// blocking, so holding it here neither deadlocks nor stalls.
	p.mu.Lock()
	defer p.mu.Unlock()

	if chain.Key != "" {
		if active := p.findActiveLocked(chain.Key); active != nil {
			return active, util.Errorf("%w", ErrTaskChainRunning)
		}
	}

	id, err := newTaskChainID()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	chain.ID = id
	if chain.CreatedAt.IsZero() {
		chain.CreatedAt = time.Now()
	}

	// Enqueue before retaining anything. A rejected chain must leave the pool
	// exactly as it found it, and evicting to make room for a chain that never
	// ran would discard a result a client can still ask for.
	if err := p.worker.Run(chain); err != nil {
		chain.ID = ""
		return nil, util.Errorf("%w", err)
	}

	p.byID[id] = chain
	p.order = append(p.order, id)
	p.evictLocked()

	return chain, nil
}

func (p *Pool) Get(id string) (*TaskChain, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chain, ok := p.byID[id]
	return chain, ok
}

// List returns the retained chains, most recently submitted first.
func (p *Pool) List() []*TaskChain {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chains := make([]*TaskChain, 0, len(p.order))
	for i := len(p.order) - 1; i >= 0; i-- {
		if chain, ok := p.byID[p.order[i]]; ok {
			chains = append(chains, chain)
		}
	}
	return chains
}

// Cancel signals the chain with the given ID to stop. Cancelling a chain that is
// still running or queued triggers its context so its tasks stop at the next
// task boundary. A chain that does not exist, or one that has already reached a
// terminal status, is left untouched. The returned result says which case
// applied, and the chain (nil only when it was not found) lets a caller report
// its current state.
func (p *Pool) Cancel(id string) (*TaskChain, CancelResult) {
	p.mu.RLock()
	chain, ok := p.byID[id]
	p.mu.RUnlock()

	if !ok {
		return nil, CancelNotFound
	}
	// A terminal chain has nothing left to stop. Reporting it separately keeps the
	// endpoint honest that the cancel changed nothing rather than implying it did.
	if chain.Status().IsTerminal() {
		return chain, CancelAlreadyTerminal
	}
	chain.Cancel()
	return chain, CancelSignalled
}

// findActiveLocked returns the retained chain with the given key that has not
// reached a terminal status yet, if any.
func (p *Pool) findActiveLocked(key string) *TaskChain {
	for _, id := range p.order {
		chain, ok := p.byID[id]
		if !ok || chain.Key != key {
			continue
		}
		if !chain.Status().IsTerminal() {
			return chain
		}
	}
	return nil
}

// evictLocked drops the oldest terminal chains until the pool is back within
// its retention budget. Active chains are never evicted, because a client may
// still be polling them.
//
// Eviction runs on submission, so the budget is a bound on what accumulates
// across submissions rather than an instantaneous cap: chains that settle after
// the last submission stay retained until the next one arrives. Since only a
// submission can add a chain, retention cannot grow without also being trimmed.
func (p *Pool) evictLocked() {
	for len(p.order) > p.maxKeep {
		evicted := false
		for _, id := range p.order {
			chain, ok := p.byID[id]
			if ok && !chain.Status().IsTerminal() {
				continue
			}
			p.removeLocked(id)
			evicted = true
			break
		}
		if !evicted {
			return
		}
	}
}

func (p *Pool) removeLocked(id string) {
	delete(p.byID, id)
	if idx := slices.Index(p.order, id); idx >= 0 {
		p.order = slices.Delete(p.order, idx, idx+1)
	}
}

func newTaskChainID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", util.Errorf("%w", err)
	}
	return hex.EncodeToString(buf), nil
}
