package taskutil

import (
	"context"
	"sync"

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

type TaskChain struct {
	Tasks []Task
}

type Worker interface {
	Run(chain *TaskChain) error

	Start()

	Close() error
}

var ErrWorkerBusy = util.NewError("worker is busy")

type worker struct {
	maxLen int
	chains chan *TaskChain
	logger *logutil.Logger

	wg sync.WaitGroup
}

func NewWorker(maxLen int, logger *logutil.Logger) Worker {
	if maxLen <= 0 {
		maxLen = 100
	}

	return &worker{
		maxLen: maxLen,
		chains: make(chan *TaskChain, maxLen),
		logger: logger,
	}
}

func (w *worker) Close() error {
	close(w.chains)
	w.wg.Wait()
	return nil
}

func (w *worker) Start() {
	w.wg.Go(w.work)
}

func (w *worker) work() {
	w.logger.Info("Worker started working")
	for chain := range w.chains {
		w.logger.Info("Worker received a task chain")
		for _, task := range chain.Tasks {
			w.logger.Info("Worker running task", "task", task.Name())
			err := task.Run(context.TODO())
			w.logger.Info("Worker finished task", "task", task.Name())
			if err != nil {
				w.logger.Error("task failed", "task", task.Name(), "error", err)
				break
			}
		}
		w.logger.Info("Worker finished task chain")
	}
}

func (w *worker) Run(chain *TaskChain) error {
	select {
	case w.chains <- chain:
		return nil

	default:
		w.logger.Error("Worker is busy, cannot accept new task chain")
		return util.Errorf("%w", ErrWorkerBusy)
	}
}
