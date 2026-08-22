package taskutil

import (
	"context"
	"errors"
	"testing"
)

// statusTask reports a status and percentage chosen by the test. It never runs
// on a worker goroutine in these tests, so it needs no locking.
type statusTask struct {
	status     Status
	percentage float64
}

func (t *statusTask) Run(ctx context.Context) error { return nil }
func (t *statusTask) Name() string                  { return "status" }
func (t *statusTask) Title() string                 { return "status" }
func (t *statusTask) Description() string           { return "status" }
func (t *statusTask) Percentage() float64           { return t.percentage }
func (t *statusTask) Status() Status                { return t.status }

func newTestPool(t *testing.T, maxLen, maxKeep int) Pool {
	t.Helper()

	p := NewPool(newTestWorker(t, maxLen), maxKeep)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Errorf("Close returned an error: %v", err)
		}
	})
	return p
}

func chainWithStatus(status Status) *TaskChain {
	return &TaskChain{Tasks: []Task{&statusTask{status: status}}}
}

func TestPoolSubmitAssignsIDAndRegistersChain(t *testing.T) {
	p := newTestPool(t, 4, 0)

	chain := chainWithStatus(StatusPending)
	submitted, err := p.Submit(chain)
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	if submitted != chain {
		t.Fatalf("Submit returned a different chain than the one submitted")
	}
	if chain.ID == "" {
		t.Fatalf("Submit did not assign an ID")
	}
	if chain.CreatedAt.IsZero() {
		t.Errorf("Submit did not set CreatedAt")
	}

	got, ok := p.Get(chain.ID)
	if !ok {
		t.Fatalf("Get(%q) reported the chain as missing", chain.ID)
	}
	if got != chain {
		t.Errorf("Get returned a different chain")
	}
}

func TestPoolSubmitAssignsUniqueIDs(t *testing.T) {
	p := newTestPool(t, 4, 0)

	first, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	second, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	if first.ID == second.ID {
		t.Errorf("Submit reused ID %q for two chains", first.ID)
	}
}

func TestPoolGetReportsUnknownID(t *testing.T) {
	p := newTestPool(t, 4, 0)

	if _, ok := p.Get("does-not-exist"); ok {
		t.Errorf("Get reported an unknown ID as present")
	}
}

func TestPoolSubmitRejectsDuplicateKeyWhileActive(t *testing.T) {
	p := newTestPool(t, 4, 0)

	running := &TaskChain{Key: "empty_trash:default", Tasks: []Task{&statusTask{status: StatusRunning}}}
	if _, err := p.Submit(running); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	duplicate := &TaskChain{Key: "empty_trash:default", Tasks: []Task{&statusTask{status: StatusPending}}}
	active, err := p.Submit(duplicate)
	if !errors.Is(err, ErrTaskChainRunning) {
		t.Fatalf("Expected ErrTaskChainRunning, got %v", err)
	}
	if active != running {
		t.Errorf("Expected the active chain to be returned so the caller can report its ID")
	}
	if duplicate.ID != "" {
		t.Errorf("Rejected chain should not be assigned an ID, got %q", duplicate.ID)
	}
}

func TestPoolSubmitAllowsSameKeyOnceTerminal(t *testing.T) {
	p := newTestPool(t, 4, 0)

	done := &TaskChain{Key: "empty_trash:default", Tasks: []Task{&statusTask{status: StatusCompleted}}}
	if _, err := p.Submit(done); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	next := &TaskChain{Key: "empty_trash:default", Tasks: []Task{&statusTask{status: StatusPending}}}
	if _, err := p.Submit(next); err != nil {
		t.Fatalf("Submit returned an error after the previous chain finished: %v", err)
	}
	if next.ID == "" || next.ID == done.ID {
		t.Errorf("Expected a fresh ID for the new chain, got %q", next.ID)
	}
}

func TestPoolSubmitAllowsConcurrentChainsWithoutKey(t *testing.T) {
	p := newTestPool(t, 4, 0)

	if _, err := p.Submit(chainWithStatus(StatusRunning)); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	if _, err := p.Submit(chainWithStatus(StatusRunning)); err != nil {
		t.Errorf("Submit rejected a keyless chain: %v", err)
	}
}

func TestPoolEvictsOldestTerminalChainsOnly(t *testing.T) {
	p := newTestPool(t, 8, 2)

	active, err := p.Submit(chainWithStatus(StatusRunning))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	oldest, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}
	newest, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	if _, ok := p.Get(oldest.ID); ok {
		t.Errorf("Expected the oldest terminal chain to be evicted")
	}
	if _, ok := p.Get(active.ID); !ok {
		t.Errorf("Expected an active chain to survive eviction, since a client may still be polling it")
	}
	if _, ok := p.Get(newest.ID); !ok {
		t.Errorf("Expected the newest chain to be retained")
	}
}

func TestPoolSubmitDoesNotRetainChainWhenWorkerRejectsIt(t *testing.T) {
	// The worker is never started, so the queue of one fills immediately.
	p := NewPool(newTestWorker(t, 1), 0)
	t.Cleanup(func() { _ = p.Close() })

	if _, err := p.Submit(chainWithStatus(StatusPending)); err != nil {
		t.Fatalf("Submit returned an error for the first chain: %v", err)
	}

	rejected := &TaskChain{Tasks: []Task{&statusTask{status: StatusPending}}}
	if _, err := p.Submit(rejected); !errors.Is(err, ErrWorkerBusy) {
		t.Fatalf("Expected ErrWorkerBusy, got %v", err)
	}
	if len(p.List()) != 1 {
		t.Errorf("Expected the rejected chain to be dropped, got %d retained chains", len(p.List()))
	}
}

// A chain whose first task ended partially completed is still being processed
// while later tasks remain, so its key must stay reserved.
func TestPoolSubmitRejectsDuplicateKeyWhileALaterTaskRemains(t *testing.T) {
	p := newTestPool(t, 4, 0)

	inFlight := &TaskChain{Key: "empty_trash:default", Tasks: []Task{
		&statusTask{status: StatusPartiallyCompleted},
		&statusTask{status: StatusPending},
	}}
	if _, err := p.Submit(inFlight); err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	duplicate := &TaskChain{Key: "empty_trash:default", Tasks: []Task{&statusTask{status: StatusPending}}}
	active, err := p.Submit(duplicate)
	if !errors.Is(err, ErrTaskChainRunning) {
		t.Fatalf("Expected ErrTaskChainRunning, got %v", err)
	}
	if active != inFlight {
		t.Errorf("Expected the in-flight chain to be returned")
	}
}

// A submission the worker rejects must leave the pool untouched, including the
// results it was already retaining.
func TestPoolSubmitRejectionDoesNotEvictRetainedChains(t *testing.T) {
	// Queue depth of 1 and retention of 1: the second submission is rejected
	// while the pool is already at its budget.
	p := NewPool(newTestWorker(t, 1), 1)
	t.Cleanup(func() { _ = p.Close() })

	retained, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	if _, err := p.Submit(chainWithStatus(StatusPending)); !errors.Is(err, ErrWorkerBusy) {
		t.Fatalf("Expected ErrWorkerBusy, got %v", err)
	}

	if _, ok := p.Get(retained.ID); !ok {
		t.Errorf("A rejected submission evicted a chain a client can still ask for")
	}
	if len(p.List()) != 1 {
		t.Errorf("Expected 1 retained chain, got %d", len(p.List()))
	}
}

func TestPoolCancelSignalsRunningChain(t *testing.T) {
	w := newTestWorker(t, 2)
	p := NewPool(w, 0)
	t.Cleanup(func() { _ = p.Close() })
	w.Start()

	started := make(chan struct{})
	observed := make(chan error, 1)
	chain, err := p.Submit(&TaskChain{Tasks: []Task{&blockingTask{started: started, observed: observed}}})
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	<-started
	got, result := p.Cancel(chain.ID)
	if result != CancelSignalled {
		t.Fatalf("Cancel result = %v, want CancelSignalled", result)
	}
	if got != chain {
		t.Errorf("Cancel returned a different chain than the one cancelled")
	}
	if err := <-observed; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled task observed %v, want context.Canceled", err)
	}
}

func TestPoolCancelUnknownIDReportsNotFound(t *testing.T) {
	p := newTestPool(t, 4, 0)

	chain, result := p.Cancel("does-not-exist")
	if result != CancelNotFound {
		t.Errorf("Cancel result = %v, want CancelNotFound", result)
	}
	if chain != nil {
		t.Errorf("Cancel returned a chain for an unknown ID: %+v", chain)
	}
}

func TestPoolCancelTerminalChainIsNoOp(t *testing.T) {
	p := newTestPool(t, 4, 0)

	done, err := p.Submit(chainWithStatus(StatusCompleted))
	if err != nil {
		t.Fatalf("Submit returned an error: %v", err)
	}

	chain, result := p.Cancel(done.ID)
	if result != CancelAlreadyTerminal {
		t.Errorf("Cancel result = %v, want CancelAlreadyTerminal", result)
	}
	if chain != done {
		t.Errorf("Cancel returned a different chain than the terminal one it was asked about")
	}
}

func TestPoolListReturnsNewestFirst(t *testing.T) {
	p := newTestPool(t, 4, 0)

	first, _ := p.Submit(chainWithStatus(StatusCompleted))
	second, _ := p.Submit(chainWithStatus(StatusCompleted))

	chains := p.List()
	if len(chains) != 2 {
		t.Fatalf("Expected 2 chains, got %d", len(chains))
	}
	if chains[0].ID != second.ID || chains[1].ID != first.ID {
		t.Errorf("Expected newest-first ordering")
	}
}
