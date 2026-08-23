package task

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const FolderTransferTaskName = "folder_transfer"

// FolderTransferBook names one book a folder transfer will carry, together with the
// source folder it sits in. The endpoint resolves the whole set up front - the same
// snapshot it screens for conflicts - and the chain constructor remaps each source
// folder onto the destination, so the task itself works from fixed destinations.
type FolderTransferBook struct {
	ID           string
	SourceFolder shelf.FolderPath
}

type folderTransferFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

// FolderTransferResult is what a client reads back from the chain as a folder
// transfer runs and once it settles. It reports the destination the transfer
// aims at and, like the batch task, the books that made it across and the ones
// that did not, so a caller sees "N of M" progress and every per-book failure.
type FolderTransferResult struct {
	Operation    string                  `json:"operation"`
	SourceShelf  string                  `json:"source_shelf"`
	TargetShelf  string                  `json:"target_shelf"`
	SourceFolder shelf.FolderPath        `json:"source_folder"`
	TargetFolder shelf.FolderPath        `json:"target_folder"`
	Total        int                     `json:"total"`
	SucceededIDs []string                `json:"succeeded_ids"`
	Failures     []folderTransferFailure `json:"failures"`

	// FolderFailures counts destination folders that could not be created. A book's
	// own folder is re-created when the book publishes, so this only ever bites an
	// empty sub-folder that has no book to rebuild it - but it still means the
	// transfer did not fully reproduce the structure, so it holds the task back
	// from a clean completion.
	FolderFailures int `json:"folder_failures"`
}

// plannedTransfer is one book paired with the destination folder it lands in,
// already remapped from its source folder onto the target root.
type plannedTransfer struct {
	id           string
	targetFolder shelf.FolderPath
}

type folderTransferTask struct {
	source    *shelf.Shelf
	target    *shelf.Shelf
	logger    *logutil.Logger
	operation string

	// sourceFolder is the folder being transferred, kept so a completed move can
	// prune the folders it emptied on the source.
	sourceFolder shelf.FolderPath
	// createFolders are the destination folders to reproduce before any book is
	// published, so an empty sub-folder carries across too.
	createFolders []shelf.FolderPath
	// books are the per-book transfers, each with its remapped destination folder,
	// sorted by ID for a deterministic transfer order.
	books []plannedTransfer

	progress taskutil.Progress
	mu       sync.Mutex
	result   FolderTransferResult
}

func newFolderTransferTask(sourceShelfID string, source *shelf.Shelf, targetShelfID string, target *shelf.Shelf, logger *logutil.Logger, operation string, sourceFolder, targetFolder shelf.FolderPath, books []FolderTransferBook, sourceSubfolders []shelf.FolderPath) *folderTransferTask {
	sourceFolder = append(shelf.FolderPath(nil), sourceFolder...)
	targetFolder = append(shelf.FolderPath(nil), targetFolder...)

	createFolders := make([]shelf.FolderPath, 0, len(sourceSubfolders))
	for _, sl := range sourceSubfolders {
		createFolders = append(createFolders, remapFolder(sl, sourceFolder, targetFolder))
	}

	planned := make([]plannedTransfer, 0, len(books))
	for _, b := range books {
		planned = append(planned, plannedTransfer{
			id:           b.ID,
			targetFolder: remapFolder(b.SourceFolder, sourceFolder, targetFolder),
		})
	}
	sort.Slice(planned, func(i, j int) bool { return planned[i].id < planned[j].id })

	return &folderTransferTask{
		source:        source,
		target:        target,
		logger:        logger,
		operation:     operation,
		sourceFolder:  sourceFolder,
		createFolders: createFolders,
		books:         planned,
		result: FolderTransferResult{
			Operation:    operation,
			SourceShelf:  sourceShelfID,
			TargetShelf:  targetShelfID,
			SourceFolder: sourceFolder,
			TargetFolder: targetFolder,
			Total:        len(planned),
			SucceededIDs: []string{},
			Failures:     []folderTransferFailure{},
		},
	}
}

func (t *folderTransferTask) Name() string { return FolderTransferTaskName }

func (t *folderTransferTask) Title() string {
	if t.operation == BookTransferOperationMove {
		return "Move a folder to another shelf"
	}
	return "Copy a folder to another shelf"
}

func (t *folderTransferTask) Description() string {
	t.mu.Lock()
	succeeded, failed, target := len(t.result.SucceededIDs), len(t.result.Failures), t.result.TargetShelf
	t.mu.Unlock()

	_, total := t.progress.Counts()
	verb := "copied"
	if t.operation == BookTransferOperationMove {
		verb = "moved"
	}
	description := verb + " " + strconv.Itoa(succeeded) + " of " + strconv.Itoa(total) + " books to " + target
	if failed > 0 {
		description += ", " + strconv.Itoa(failed) + " failed"
	}
	return description
}

func (t *folderTransferTask) Percentage() float64 { return t.progress.Percentage() }

func (t *folderTransferTask) Status() taskutil.Status { return t.progress.Status() }

func (t *folderTransferTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return FolderTransferResult{
		Operation:      t.result.Operation,
		SourceShelf:    t.result.SourceShelf,
		TargetShelf:    t.result.TargetShelf,
		SourceFolder:   append(shelf.FolderPath(nil), t.result.SourceFolder...),
		TargetFolder:   append(shelf.FolderPath(nil), t.result.TargetFolder...),
		Total:          t.result.Total,
		SucceededIDs:   append(make([]string, 0, len(t.result.SucceededIDs)), t.result.SucceededIDs...),
		Failures:       append(make([]folderTransferFailure, 0, len(t.result.Failures)), t.result.Failures...),
		FolderFailures: t.result.FolderFailures,
	}
}

func (t *folderTransferTask) recordSuccess(bookID string) {
	t.mu.Lock()
	t.result.SucceededIDs = append(t.result.SucceededIDs, bookID)
	t.mu.Unlock()
}

func (t *folderTransferTask) recordFailure(bookID string, err error) {
	code := failureCode(t.operation, err)
	t.mu.Lock()
	t.result.Failures = append(t.result.Failures, folderTransferFailure{BookID: bookID, Code: code})
	t.mu.Unlock()
	t.logger.Error("folder transfer item failed",
		"operation", t.operation,
		"source_shelf", t.result.SourceShelf,
		"target_shelf", t.result.TargetShelf,
		"book_id", bookID,
		"error", err)
}

// interruptedStatus classifies a run stopped by a cancel: failed if it never
// published a book, partially completed if some already landed - the same split
// the batch task draws, so a cancel never reads as a clean completion.
func (t *folderTransferTask) interruptedStatus() taskutil.Status {
	t.mu.Lock()
	processed := len(t.result.SucceededIDs) + len(t.result.Failures)
	t.mu.Unlock()
	if processed == 0 {
		return taskutil.StatusFailed
	}
	return taskutil.StatusPartiallyCompleted
}

func (t *folderTransferTask) finishStatus() taskutil.Status {
	t.mu.Lock()
	succeeded, failed := len(t.result.SucceededIDs), len(t.result.Failures)
	t.mu.Unlock()

	folderFailures := t.folderFailures()
	switch {
	case failed == 0 && folderFailures == 0:
		return taskutil.StatusCompleted
	case succeeded == 0:
		return taskutil.StatusFailed
	default:
		return taskutil.StatusPartiallyCompleted
	}
}

func (t *folderTransferTask) folderFailures() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result.FolderFailures
}

func (t *folderTransferTask) Run(ctx context.Context) error {
	t.progress.SetTotal(len(t.books))
	t.progress.SetStatus(taskutil.StatusRunning)

	// Refuse before any work when the transfer is cancelled up front, so a cancel
	// that arrives before the first book leaves both shelves untouched.
	if err := ctx.Err(); err != nil {
		t.progress.SetStatus(t.interruptedStatus())
		return util.Errorf("%w", err)
	}

	// Reproduce the destination folder tree first - empty sub-folders included -
	// so the structure carries across even where there is no book to publish.
	// Publishing a book re-creates its own folder, so re-creating one here is
	// harmless and a book's folder survives a failure here. A failure only strands
	// an empty folder, so it does not abort the transfer, but it is counted so the
	// task settles short of a clean completion rather than claiming it copied a
	// structure it did not.
	for _, folder := range t.createFolders {
		parent := append(shelf.FolderPath(nil), folder[:len(folder)-1]...)
		if err := t.target.NewFolder(parent, folder[len(folder)-1]); err != nil {
			t.mu.Lock()
			t.result.FolderFailures++
			t.mu.Unlock()
			t.logger.Error("folder transfer could not create a target folder",
				"target_shelf", t.result.TargetShelf, "folder", folder.String(), "error", err)
		}
	}

	for _, planned := range t.books {
		// Checked before each book so a cancel stops the transfer between books
		// rather than partway through one: a half-copied book is never published.
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(t.interruptedStatus())
			return util.Errorf("%w", err)
		}

		var err error
		if t.operation == BookTransferOperationMove {
			_, err = t.target.MoveBookFrom(t.source, planned.id, planned.targetFolder)
		} else {
			_, err = t.target.CopyBookFrom(t.source, planned.id, planned.targetFolder, false)
		}
		if err != nil {
			t.recordFailure(planned.id, err)
		} else {
			t.recordSuccess(planned.id)
		}
		t.progress.Advance()
	}

	// A move that ran to the end leaves the source subtree holding only empty
	// folders (and any package the scan could not read, which was never in the
	// plan); drop the empty ones so the moved folder no longer shows on the source.
	// This is reached only when the loop was not cancelled, so a cancel keeps the
	// partially emptied source exactly as it is.
	if t.operation == BookTransferOperationMove {
		t.pruneSourceFolder()
	}

	t.progress.SetStatus(t.finishStatus())
	return nil
}

// pruneSourceFolder removes the now-empty folders a completed move left under the
// source folder. It leans on DeleteFolder, which refuses a non-empty folder, so a
// book that failed to move (still on the source) or a package the scan could not
// read keeps its folder - the move never deletes anything it did not carry.
// Deepest folders first, so a parent is empty by the time it is reached.
func (t *folderTransferTask) pruneSourceFolder() {
	folders, err := t.source.GetAllFolders()
	if err != nil {
		t.logger.Error("folder transfer could not list source folders to prune",
			"source_shelf", t.result.SourceShelf, "folder", t.sourceFolder.String(), "error", err)
		return
	}

	under := make([]shelf.FolderPath, 0, len(folders))
	for _, l := range folders {
		if folderHasPrefix(l, t.sourceFolder) {
			under = append(under, l)
		}
	}
	sort.Slice(under, func(i, j int) bool { return len(under[i]) > len(under[j]) })

	for _, l := range under {
		// A non-empty folder (a failed book or an unreadable package) is left in
		// place, exactly as the shelf's own folder move does.
		_ = t.source.DeleteFolder(l)
	}
}

// NewFolderTransferChain builds the one-task chain that copies or moves a whole
// folder from one shelf to another. The dedupe key folds in both shelves, the
// operation and both folders, so re-submitting the same transfer attaches to the
// one already running instead of starting a second.
func NewFolderTransferChain(sourceShelfID string, source *shelf.Shelf, targetShelfID string, target *shelf.Shelf, logger *logutil.Logger, operation string, sourceFolder, targetFolder shelf.FolderPath, books []FolderTransferBook, sourceSubfolders []shelf.FolderPath) *taskutil.TaskChain {
	key := strings.Join([]string{
		FolderTransferTaskName, sourceShelfID, targetShelfID, operation, sourceFolder.String(), targetFolder.String(),
	}, ":")
	task := newFolderTransferTask(sourceShelfID, source, targetShelfID, target, logger, operation, sourceFolder, targetFolder, books, sourceSubfolders)
	return &taskutil.TaskChain{
		Key:         key,
		Name:        FolderTransferTaskName,
		Title:       task.Title(),
		Description: "Transfer a folder to another shelf",
		Tasks:       []taskutil.Task{task},
	}
}

// folderHasPrefix reports whether folder is prefix itself or sits beneath it.
func folderHasPrefix(folder, prefix shelf.FolderPath) bool {
	if len(folder) < len(prefix) {
		return false
	}
	for i := range prefix {
		if folder[i] != prefix[i] {
			return false
		}
	}
	return true
}

// remapFolder rewrites a source folder sitting under sourceFolder into its place
// under targetFolder, preserving the tail below sourceFolder. sourceFolder must be a
// prefix of folder, which the caller has already ensured for every folder it maps.
func remapFolder(folder, sourceFolder, targetFolder shelf.FolderPath) shelf.FolderPath {
	tail := folder[len(sourceFolder):]
	mapped := append(append(shelf.FolderPath(nil), targetFolder...), tail...)
	return mapped
}
