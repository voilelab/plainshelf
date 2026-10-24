package task

import (
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func settleFinishedSweep(progress *taskutil.Progress, failed int) error {
	if failed > 0 {
		progress.SetStatus(taskutil.StatusPartiallyCompleted)
	} else {
		progress.SetStatus(taskutil.StatusCompleted)
	}
	return nil
}

func snapshotBookIDs(books []*shelf.Book) []string {
	ids := make([]string, 0, len(books))
	for _, book := range books {
		ids = append(ids, book.ID())
	}
	return ids
}
