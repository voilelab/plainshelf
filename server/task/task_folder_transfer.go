package task

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const FolderTransferTaskName = "folder_transfer"

type FolderTransferBook struct {
	ID           string
	SourceFolder shelf.FolderPath
}

type folderTransferFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

type FolderTransferResult struct {
	Operation    string                  `json:"operation"`
	SourceShelf  string                  `json:"source_shelf"`
	TargetShelf  string                  `json:"target_shelf"`
	SourceFolder shelf.FolderPath        `json:"source_folder"`
	TargetFolder shelf.FolderPath        `json:"target_folder"`
	Total        int                     `json:"total"`
	SucceededIDs []string                `json:"succeeded_ids"`
	Failures     []folderTransferFailure `json:"failures"`

	FolderFailures int `json:"folder_failures"`
}

type plannedFolder struct {
	source shelf.FolderPath
	target shelf.FolderPath
}

type plannedTransfer struct {
	id           string
	targetFolder shelf.FolderPath
}

type folderTransferTask struct {
	source    *shelf.Shelf
	target    *shelf.Shelf
	logger    *logutil.Logger
	operation string

	sourceFolder shelf.FolderPath

	createFolders []plannedFolder

	carriedMeta carriedFolderMeta

	books []plannedTransfer

	progress taskutil.Progress
	mu       sync.Mutex
	result   FolderTransferResult
}

func newFolderTransferTask(sourceShelfID string, source *shelf.Shelf, targetShelfID string, target *shelf.Shelf, logger *logutil.Logger, operation string, sourceFolder, targetFolder shelf.FolderPath, books []FolderTransferBook, sourceSubfolders []shelf.FolderPath) *folderTransferTask {
	sourceFolder = append(shelf.FolderPath(nil), sourceFolder...)
	targetFolder = append(shelf.FolderPath(nil), targetFolder...)

	createFolders := make([]plannedFolder, 0, len(sourceSubfolders))
	for _, sl := range sourceSubfolders {
		createFolders = append(createFolders, plannedFolder{source: sl, target: remapFolder(sl, sourceFolder, targetFolder)})
	}

	planned := make([]plannedTransfer, 0, len(books))
	for _, b := range books {
		planned = append(planned, plannedTransfer{
			id:           b.ID,
			targetFolder: remapFolder(b.SourceFolder, sourceFolder, targetFolder),
		})
	}
	slices.SortFunc(planned, func(a, b plannedTransfer) int { return cmp.Compare(a.id, b.id) })

	return &folderTransferTask{
		source:        source,
		target:        target,
		logger:        logger,
		operation:     operation,
		sourceFolder:  sourceFolder,
		createFolders: createFolders,
		carriedMeta:   make(carriedFolderMeta, len(createFolders)),
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

	if err := ctx.Err(); err != nil {
		return t.interrupt(err)
	}
	t.reproduceFolderTree()
	if err := t.transferBooks(ctx); err != nil {
		return t.interrupt(err)
	}
	if t.operation == BookTransferOperationMove {
		t.pruneEmptiedSourceFolders()
	}

	t.progress.SetStatus(t.finishStatus())
	return nil
}

func (t *folderTransferTask) interrupt(cause error) error {
	t.progress.SetStatus(t.interruptedStatus())
	return util.Errorf("%w", cause)
}

func (t *folderTransferTask) reproduceFolderTree() {
	for _, folder := range t.createFolders {
		if err := t.createFolder(folder); err != nil {
			t.mu.Lock()
			t.result.FolderFailures++
			t.mu.Unlock()
			t.logger.Error("folder transfer could not reproduce a target folder",
				"target_shelf", t.result.TargetShelf, "folder", folder.target.String(), "error", err)
		}
	}
}

func (t *folderTransferTask) transferBooks(ctx context.Context) error {
	for _, planned := range t.books {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := transferAtomically(t.operation, t.source, t.target, planned.id, planned.targetFolder); err != nil {
			t.recordFailure(planned.id, err)
		} else {
			t.recordSuccess(planned.id)
		}
		t.progress.Advance()
	}
	return nil
}

func (t *folderTransferTask) createFolder(folder plannedFolder) error {
	parent := append(shelf.FolderPath(nil), folder.target[:len(folder.target)-1]...)
	if err := t.target.NewFolder(parent, folder.target[len(folder.target)-1]); err != nil {
		return util.Errorf("%w", err)
	}
	version, err := t.target.CopyFolderMetaFrom(t.source, folder.source, folder.target)
	if err != nil {
		return util.Errorf("%w", err)
	}
	t.carriedMeta[folder.source.String()] = version
	return nil
}

type carriedFolderMeta map[string]string

func (c carriedFolderMeta) versionSafeToDrop(folder shelf.FolderPath) string {
	return c[folder.String()]
}

func deepestFirst(folders []shelf.FolderPath) {
	slices.SortFunc(folders, func(a, b shelf.FolderPath) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a.String(), b.String()))
	})
}

func (t *folderTransferTask) pruneEmptiedSourceFolders() {
	folders, err := t.source.GetAllFolders()
	if err != nil {
		t.logger.Error("folder transfer could not list source folders to prune",
			"source_shelf", t.result.SourceShelf, "folder", t.sourceFolder.String(), "error", err)
		return
	}

	under := make([]shelf.FolderPath, 0, len(folders))
	for _, l := range folders {
		if l.HasPrefix(t.sourceFolder) {
			under = append(under, l)
		}
	}

	deepestFirst(under)
	for _, l := range under {
		_ = t.source.PruneFolder(l, t.carriedMeta.versionSafeToDrop(l))
	}
}

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

func remapFolder(folder, sourceFolder, targetFolder shelf.FolderPath) shelf.FolderPath {
	return slices.Concat(targetFolder, folder[len(sourceFolder):])
}
