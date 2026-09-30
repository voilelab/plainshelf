package taskutil

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
)

// fakeTask appends its name to a shared record when it runs, and optionally fails.
// Tasks only run on the single worker goroutine, and Close synchronizes with it,
// so the record needs no extra locking.
type fakeTask struct {
	name   string
	err    error
	record *[]string
}

func (t *fakeTask) Run(ctx context.Context) error {
	*t.record = append(*t.record, t.name)
	return t.err
}

func (t *fakeTask) Name() string        { return t.name }
func (t *fakeTask) Title() string       { return t.name }
func (t *fakeTask) Description() string { return t.name }
func (t *fakeTask) Percentage() float64 { return 0 }
func (t *fakeTask) Status() Status      { return StatusPending }

// blockingTask reports when it starts, then waits for the worker context to be
// cancelled and publishes the error it observed.
type blockingTask struct {
	started  chan struct{}
	observed chan error
}

func (t *blockingTask) Run(ctx context.Context) error {
	close(t.started)
	<-ctx.Done()
	t.observed <- ctx.Err()
	return ctx.Err()
}

func (t *blockingTask) Name() string        { return "blocking" }
func (t *blockingTask) Title() string       { return "blocking" }
func (t *blockingTask) Description() string { return "blocking" }
func (t *blockingTask) Percentage() float64 { return 0 }
func (t *blockingTask) Status() Status      { return StatusRunning }

func newTestWorker(t *testing.T, maxLen int) *Worker {
	t.Helper()

	logger, err := logutil.NewLogger(&logutil.LogConf{
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
	})
	if err != nil {
		t.Fatalf("NewLogger returned an error: %v", err)
	}

	return NewWorker(maxLen, logger)
}

func TestWorkerRunsChainInOrder(t *testing.T) {
	w := newTestWorker(t, 2)
	w.Start()

	ran := []string{}
	chain := &TaskChain{Tasks: []Task{
		&fakeTask{name: "first", record: &ran},
		&fakeTask{name: "second", record: &ran},
		&fakeTask{name: "third", record: &ran},
	}}

	if err := w.Run(chain); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	// Close waits for the worker goroutine to drain the queue.
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}

	want := []string{"first", "second", "third"}
	if !slices.Equal(ran, want) {
		t.Errorf("Expected tasks %v to run, got %v", want, ran)
	}
}

func TestWorkerStopsChainAfterFailure(t *testing.T) {
	w := newTestWorker(t, 2)
	w.Start()

	ran := []string{}
	chain := &TaskChain{Tasks: []Task{
		&fakeTask{name: "first", record: &ran},
		&fakeTask{name: "failing", record: &ran, err: errors.New("boom")},
		&fakeTask{name: "skipped", record: &ran},
	}}

	if err := w.Run(chain); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}

	want := []string{"first", "failing"}
	if !slices.Equal(ran, want) {
		t.Errorf("Expected tasks %v to run, got %v", want, ran)
	}
}

func TestWorkerCancelsRunningTaskOnClose(t *testing.T) {
	w := newTestWorker(t, 1)
	w.Start()

	started := make(chan struct{})
	observed := make(chan error, 1)
	chain := &TaskChain{Tasks: []Task{&blockingTask{started: started, observed: observed}}}

	if err := w.Run(chain); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	<-started
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}

	if err := <-observed; !errors.Is(err, context.Canceled) {
		t.Errorf("Expected the running task to observe context.Canceled, got %v", err)
	}
}

func TestWorkerCancelStopsOnlyThatChain(t *testing.T) {
	w := newTestWorker(t, 2)
	w.Start()

	started := make(chan struct{})
	observed := make(chan error, 1)
	cancelled := &TaskChain{Tasks: []Task{&blockingTask{started: started, observed: observed}}}

	if err := w.Run(cancelled); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	<-started
	// Cancelling one chain must stop its task without taking the worker down.
	cancelled.Cancel()
	if err := <-observed; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled task observed %v, want context.Canceled", err)
	}

	// The worker is still alive: a chain submitted after the cancel runs to the end.
	ran := []string{}
	next := &TaskChain{Tasks: []Task{&fakeTask{name: "after", record: &ran}}}
	if err := w.Run(next); err != nil {
		t.Fatalf("Run after cancel returned an error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}

	if !slices.Equal(ran, []string{"after"}) {
		t.Errorf("worker stopped after a per-chain cancel, ran %v", ran)
	}
}

func TestWorkerRunAfterCloseReturnsError(t *testing.T) {
	w := newTestWorker(t, 1)
	w.Start()

	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}

	ran := []string{}
	err := w.Run(&TaskChain{Tasks: []Task{&fakeTask{name: "late", record: &ran}}})
	if !errors.Is(err, ErrWorkerClosed) {
		t.Errorf("Expected ErrWorkerClosed, got %v", err)
	}
}

func TestWorkerCloseIsIdempotent(t *testing.T) {
	w := newTestWorker(t, 1)
	w.Start()

	if err := w.Close(); err != nil {
		t.Fatalf("first Close returned an error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close returned an error: %v", err)
	}
}

func TestWorkerRunReturnsErrWorkerBusyWhenQueueIsFull(t *testing.T) {
	// The worker is not started, so nothing drains the queue.
	w := newTestWorker(t, 1)

	ran := []string{}
	if err := w.Run(&TaskChain{Tasks: []Task{&fakeTask{name: "queued", record: &ran}}}); err != nil {
		t.Fatalf("Run returned an error for the first chain: %v", err)
	}

	err := w.Run(&TaskChain{Tasks: []Task{&fakeTask{name: "rejected", record: &ran}}})
	if !errors.Is(err, ErrWorkerBusy) {
		t.Errorf("Expected ErrWorkerBusy, got %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}
	if len(ran) != 0 {
		t.Errorf("Expected no task to run, got %v", ran)
	}
}
