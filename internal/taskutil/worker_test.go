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

func newTestWorker(t *testing.T, maxLen int) *worker {
	t.Helper()

	logger, err := logutil.NewLogger(&logutil.LogConf{
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
	})
	if err != nil {
		t.Fatalf("NewLogger returned an error: %v", err)
	}

	w, ok := NewWorker(maxLen, logger).(*worker)
	if !ok {
		t.Fatalf("NewWorker did not return *worker")
	}
	return w
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
