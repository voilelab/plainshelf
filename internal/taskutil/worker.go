package taskutil

import (
	"context"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusPartiallyCompleted
	StatusCompleted
	StatusFailed
)

// String returns the stable wire name of the status. Callers that serialize a
// status must use this instead of the underlying integer.
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusPartiallyCompleted:
		return "partially_completed"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// IsTerminal reports whether no further progress is expected for the status.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusPartiallyCompleted, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}

type Task interface {
	Run(ctx context.Context) error

	Name() string
	Title() string
	Description() string

	// Percentage returns the completion percentage of the task.
	Percentage() float64

	// Status returns the current status of the task.
	Status() Status
}

// ResultProvider is implemented by tasks that expose a structured, read-only
// result through the task-chain API. Result must return a snapshot that callers
// can safely serialize while the task is still running.
type ResultProvider interface {
	Result() any
}

type TaskChain struct {
	// ID identifies the chain once it has been submitted to a Pool. It is
	// assigned by the pool; callers leave it empty.
	ID string

	// Key optionally deduplicates chains. A pool refuses to submit a chain
	// whose key matches an already active chain. An empty key disables the
	// check.
	Key string

	Name        string
	Title       string
	Description string

	CreatedAt time.Time

	Tasks []Task

	// mu guards the per-chain cancellation state below. It is independent of the
	// aggregate status reads, which take no lock.
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// attachContext derives the chain's own cancellable context from parent and
// stores its cancel so Cancel can stop just this chain. The worker calls it
// before the chain is queued, so a cancel that arrives once the chain is
// addressable always finds a cancel to trigger.
func (c *TaskChain) attachContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.ctx = ctx
	c.cancel = cancel
	c.mu.Unlock()
	return ctx
}

// runContext returns the context the chain's tasks run under, falling back to
// the background context for a chain that was never attached (only the tests
// that run a chain without a worker hit that path).
func (c *TaskChain) runContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Cancel triggers the chain's own context cancellation. It is safe to call more
// than once and is a no-op for a chain whose context was never attached; the
// chain's tasks observe it through ctx.Err() and stop between task boundaries.
func (c *TaskChain) Cancel() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Status aggregates the status of every task in the chain.
//
// A failed task makes the chain failed immediately, because the worker abandons
// the rest of the chain once a task returns an error. Any other outcome is only
// reported once every task has settled: a chain with work still ahead of it is
// running, never terminal, so that a pool does not release its key or evict it
// while the worker is still processing it.
//
// An empty chain has nothing left to do and reports StatusCompleted.
func (c *TaskChain) Status() Status {
	if len(c.Tasks) == 0 {
		return StatusCompleted
	}

	settled := 0
	running := false
	partial := false
	for _, task := range c.Tasks {
		switch task.Status() {
		case StatusFailed:
			return StatusFailed
		case StatusPartiallyCompleted:
			partial = true
			settled++
		case StatusCompleted:
			settled++
		case StatusRunning:
			running = true
		}
	}

	if settled < len(c.Tasks) {
		if running || settled > 0 {
			return StatusRunning
		}
		return StatusPending
	}

	if partial {
		return StatusPartiallyCompleted
	}
	return StatusCompleted
}

// Percentage averages the completion of every task in the chain, weighting each
// task equally. The result is clamped to the 0-100 range.
func (c *TaskChain) Percentage() float64 {
	if len(c.Tasks) == 0 {
		return 100
	}

	total := 0.0
	for _, task := range c.Tasks {
		total += min(max(task.Percentage(), 0), 100)
	}
	return total / float64(len(c.Tasks))
}

var ErrWorkerBusy = util.NewError("worker is busy")

var ErrWorkerClosed = util.NewError("worker is closed")

// Worker runs submitted task chains one at a time on a single goroutine, in the
// order they were queued.
type Worker struct {
	maxLen int
	chains chan *TaskChain
	logger *logutil.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	closed bool

	wg sync.WaitGroup
}

// NewWorker returns a worker whose queue holds at most maxLen chains.
func NewWorker(maxLen int, logger *logutil.Logger) *Worker {
	if maxLen <= 0 {
		maxLen = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		maxLen: maxLen,
		chains: make(chan *TaskChain, maxLen),
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (w *Worker) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	close(w.chains)
	w.mu.Unlock()

	// Cancel before waiting so a task that is already running observes the
	// shutdown instead of blocking Close for its full duration.
	w.cancel()
	w.wg.Wait()
	return nil
}

func (w *Worker) Start() {
	w.wg.Go(w.work)
}

func (w *Worker) work() {
	w.logger.Info("Worker started working")
	for chain := range w.chains {
		w.logger.Info("Worker received a task chain")
		ctx := chain.runContext()
		for _, task := range chain.Tasks {
			w.logger.Info("Worker running task", "task", task.Name())
			err := task.Run(ctx)
			w.logger.Info("Worker finished task", "task", task.Name())
			if err != nil {
				w.logger.Error("task failed", "task", task.Name(), "error", err)
				break
			}
		}
		// Release the chain's context once its tasks have settled so the child it
		// registered on the worker's shutdown context does not linger.
		chain.Cancel()
		w.logger.Info("Worker finished task chain")
	}
}

func (w *Worker) Run(chain *TaskChain) error {
	// Holding the lock keeps Close from closing the channel between the check
	// and the send, which would otherwise panic.
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return util.Errorf("%w", ErrWorkerClosed)
	}

	// Give the chain its own cancellable context, derived from the worker's
	// shutdown context, before it is queued. Deriving it here rather than when the
	// worker dequeues the chain closes the window where the chain is already
	// registered with the pool - and so cancellable - but has no cancel to trigger.
	// Closing the worker still cancels every chain, because each derives from w.ctx.
	chain.attachContext(w.ctx)

	select {
	case w.chains <- chain:
		return nil

	default:
		// The chain never made it onto the queue, so release the context we just
		// derived instead of leaking the child it registered on w.ctx.
		chain.Cancel()
		w.logger.Error("Worker is busy, cannot accept new task chain")
		return util.Errorf("%w", ErrWorkerBusy)
	}
}
