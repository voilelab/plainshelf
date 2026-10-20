package taskutil

import (
	"crypto/rand"
	"slices"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

const defaultMaxKeep = 100

var ErrTaskChainRunning = util.NewError("task chain is already running")

type CancelResult int

const (
	CancelSignalled CancelResult = iota
	CancelNotFound
	CancelAlreadyTerminal
)

type Pool struct {
	worker  *Worker
	maxKeep int

	mu    sync.RWMutex
	byID  map[string]*TaskChain
	order []string
}

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

func (p *Pool) Submit(chain *TaskChain) (*TaskChain, error) {
	if chain == nil {
		return nil, util.Errorf("task chain cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if chain.Key != "" {
		if active := p.findActiveLocked(chain.Key); active != nil {
			return active, util.Errorf("%w", ErrTaskChainRunning)
		}
	}

	id := rand.Text()
	chain.ID = id
	if chain.CreatedAt.IsZero() {
		chain.CreatedAt = time.Now()
	}

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

func (p *Pool) List() []*TaskChain {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chains := make([]*TaskChain, 0, len(p.order))
	for _, v := range slices.Backward(p.order) {
		if chain, ok := p.byID[v]; ok {
			chains = append(chains, chain)
		}
	}
	return chains
}

func (p *Pool) Busy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, chain := range p.byID {
		if !chain.Status().IsTerminal() {
			return true
		}
	}
	return false
}

func (p *Pool) Cancel(id string) (*TaskChain, CancelResult) {
	p.mu.RLock()
	chain, ok := p.byID[id]
	p.mu.RUnlock()

	if !ok {
		return nil, CancelNotFound
	}
	if chain.Status().IsTerminal() {
		return chain, CancelAlreadyTerminal
	}
	chain.Cancel()
	return chain, CancelSignalled
}

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
