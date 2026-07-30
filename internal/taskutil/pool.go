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

// Pool submits task chains to a worker and keeps them addressable by ID so that
// callers can poll their status after submission.
//
// The pool owns the worker: Start and Close delegate to it.
type Pool interface {
	// Submit registers the chain, assigns it an ID, and enqueues it.
	//
	// When the chain carries a Key that matches an active chain, the existing
	// chain is returned together with ErrTaskChainRunning so that the caller can
	// point the client at the work already in flight. Otherwise the submitted
	// chain is returned.
	Submit(chain *TaskChain) (*TaskChain, error)

	Get(id string) (*TaskChain, bool)

	// List returns the retained chains, most recently submitted first.
	List() []*TaskChain

	Start()

	Close() error
}

type pool struct {
	worker  Worker
	maxKeep int

	mu sync.RWMutex
	// byID holds every retained chain; order lists their IDs oldest first.
	byID  map[string]*TaskChain
	order []string
}

// NewPool returns a pool backed by w, retaining at most maxKeep chains.
func NewPool(w Worker, maxKeep int) Pool {
	if maxKeep <= 0 {
		maxKeep = defaultMaxKeep
	}

	return &pool{
		worker:  w,
		maxKeep: maxKeep,
		byID:    map[string]*TaskChain{},
	}
}

func (p *pool) Start() { p.worker.Start() }

func (p *pool) Close() error { return p.worker.Close() }

func (p *pool) Submit(chain *TaskChain) (*TaskChain, error) {
	if chain == nil {
		return nil, util.Errorf("task chain cannot be nil")
	}

	p.mu.Lock()

	if chain.Key != "" {
		if active := p.findActiveLocked(chain.Key); active != nil {
			p.mu.Unlock()
			return active, util.Errorf("%w", ErrTaskChainRunning)
		}
	}

	id, err := newTaskChainID()
	if err != nil {
		p.mu.Unlock()
		return nil, util.Errorf("%w", err)
	}

	chain.ID = id
	if chain.CreatedAt.IsZero() {
		chain.CreatedAt = time.Now()
	}

	p.byID[id] = chain
	p.order = append(p.order, id)
	p.evictLocked()
	p.mu.Unlock()

	if err := p.worker.Run(chain); err != nil {
		// The chain never entered the queue, so do not retain it.
		p.mu.Lock()
		p.removeLocked(id)
		p.mu.Unlock()
		return nil, util.Errorf("%w", err)
	}

	return chain, nil
}

func (p *pool) Get(id string) (*TaskChain, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chain, ok := p.byID[id]
	return chain, ok
}

func (p *pool) List() []*TaskChain {
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

// findActiveLocked returns the retained chain with the given key that has not
// reached a terminal status yet, if any.
func (p *pool) findActiveLocked(key string) *TaskChain {
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
// its retention budget. Active chains are never evicted, because a client is
// still polling them.
func (p *pool) evictLocked() {
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

func (p *pool) removeLocked(id string) {
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
