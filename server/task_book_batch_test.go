package server

import (
	"context"
	"errors"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func batchTaskForTest(t *testing.T, env *apiTestEnv, operation string, ids []string, target shelf.Layers) *bookBatchTask {
	t.Helper()
	shelfData, ok := env.app.shelfManager.GetShelf("default_shelf")
	if !ok {
		t.Fatal("default shelf is unavailable")
	}
	return newBookBatchTask("default_shelf", shelfData.Shelf, &env.app.Logger, operation, ids, target)
}

func TestBookBatchTaskNoOpStillCompletes(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Already there", "target", "already.txt", "body")
	task := batchTaskForTest(t, env, bookBatchOperationMove, []string{book.Meta.ID}, shelf.Layers{"target"})

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}
	result := task.Result().(bookBatchResult)
	if len(result.SucceededIDs) != 1 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want one no-op success", result)
	}
}

func TestBookBatchTaskAllFailuresStillProcessesEveryBook(t *testing.T) {
	env := newAPITestEnv(t)
	task := batchTaskForTest(t, env, bookBatchOperationTrash, []string{"missing-a", "missing-b"}, nil)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run returned an item error: %v", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want failed at 100%%", task.Status(), task.Percentage())
	}
	result := task.Result().(bookBatchResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 2 {
		t.Fatalf("result = %+v, want two failures", result)
	}
	for _, failure := range result.Failures {
		if failure.Code != "not_found" {
			t.Errorf("failure code = %q, want not_found", failure.Code)
		}
	}
}

func TestBookBatchTaskCancelledBeforeStart(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Cancelled", "", "cancelled.txt", "body")
	task := batchTaskForTest(t, env, bookBatchOperationTrash, []string{book.Meta.ID}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := task.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 0 {
		t.Fatalf("status = %v at %v%%, want failed at 0%%", task.Status(), task.Percentage())
	}
	result := task.Result().(bookBatchResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 0 {
		t.Fatalf("cancelled task recorded item results: %+v", result)
	}

	shelfData, _ := env.app.shelfManager.GetShelf("default_shelf")
	if _, err := shelfData.GetBook(book.Meta.ID); err != nil {
		t.Fatalf("cancelled task changed the book: %v", err)
	}
}

func TestBookBatchTaskResultSnapshotsAreIndependent(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Snapshot", "", "snapshot.txt", "body")
	task := batchTaskForTest(t, env, bookBatchOperationMove, []string{book.Meta.ID}, nil)
	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	first := task.Result().(bookBatchResult)
	first.SucceededIDs[0] = "mutated"
	first.Failures = append(first.Failures, bookBatchFailure{BookID: "fake", Code: "move_failed"})
	second := task.Result().(bookBatchResult)
	if len(second.SucceededIDs) != 1 || second.SucceededIDs[0] != book.Meta.ID || len(second.Failures) != 0 {
		t.Fatalf("Result shared mutable storage: %+v", second)
	}
}
