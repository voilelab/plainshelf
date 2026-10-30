package server

import (
	"context"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

type storeAfterCancelTask struct {
	taskutil.Progress
	app     *App
	started chan struct{}
	err     chan error
}

func (t *storeAfterCancelTask) Run(ctx context.Context) error {
	close(t.started)
	<-ctx.Done()
	err := t.app.storeDB.SetSetting("close_order_probe", []byte("1"))
	t.err <- err
	return err
}

func (*storeAfterCancelTask) Name() string        { return "store-after-cancel" }
func (*storeAfterCancelTask) Title() string       { return "" }
func (*storeAfterCancelTask) Description() string { return "" }

func TestAppCloseStopsTasksBeforeStore(t *testing.T) {
	app, err := NewApp(&AppConf{
		Shelves:   []*shelf.ShelfConfWithID{{ID: "default_shelf", LibRoot: t.TempDir()}},
		StorePath: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}

	task := &storeAfterCancelTask{app: app, started: make(chan struct{}), err: make(chan error, 1)}
	if _, err := app.taskChains.Submit(&taskutil.TaskChain{Tasks: []taskutil.Task{task}}); err != nil {
		t.Fatal(err)
	}
	<-task.started

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := <-task.err; err != nil {
		t.Fatalf("store write from a task finishing after cancel = %v, want the store still open", err)
	}
}
