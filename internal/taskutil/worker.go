package taskutil

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
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

	Percentage() float64

	Status() Status
}

type ResultProvider interface {
	Result() any
}

type TaskChain struct {
	ID string

	Key string

	RequestID string

	Name        string
	Title       string
	Description string

	CreatedAt time.Time

	Tasks []Task

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	panicked atomic.Bool
}

func (c *TaskChain) attachContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.ctx = ctx
	c.cancel = cancel
	c.mu.Unlock()
	return ctx
}

func (c *TaskChain) runContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *TaskChain) Cancel() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *TaskChain) Status() Status {
	if c.panicked.Load() {
		return StatusFailed
	}
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
			err := runTask(ctx, chain, task)
			w.logger.Info("Worker finished task", "task", task.Name())
			if err != nil {
				w.logger.Error("task failed", "request_id", chain.RequestID, "task", task.Name(), "error", err)
				break
			}
		}
		chain.Cancel()
		w.logger.Info("Worker finished task chain")
	}
}

func runTask(ctx context.Context, chain *TaskChain, task Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			chain.panicked.Store(true)
			err = util.Errorf("task panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return task.Run(ctx)
}

func (w *Worker) Run(chain *TaskChain) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return util.Errorf("%w", ErrWorkerClosed)
	}

	chain.attachContext(w.ctx)

	select {
	case w.chains <- chain:
		return nil

	default:
		chain.Cancel()
		w.logger.Error("Worker is busy, cannot accept new task chain")
		return util.Errorf("%w", ErrWorkerBusy)
	}
}
