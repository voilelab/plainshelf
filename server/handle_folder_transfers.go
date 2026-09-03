package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// folderTransferHandlers serves the endpoint that copies or moves a whole folder -
// the folder and every book and sub-folder beneath it - from one shelf to
// another. Like the single-book transfer it starts a background chain, because a
// folder can hold hundreds of megabytes and one end may be a network mount, so the
// request must not block on the copy.
type folderTransferHandlers struct {
	*taskSubmitter
}

// folderTransferRequest names the source folder and the destination of a
// cross-shelf folder transfer. It mirrors the single-book transfer body, with the
// source folder moved into the body because a folder path holds slashes and does
// not sit cleanly in a path segment - the same reason the same-shelf folder move
// carries its folder in the body. TargetFolder is a pointer so an omitted folder
// (transfer keeping the same folder name) is distinct from an empty one.
type folderTransferRequest struct {
	Mode         string            `json:"mode"`
	SourceFolder shelf.FolderPath  `json:"source_folder"`
	TargetShelf  string            `json:"target_shelf"`
	TargetFolder *shelf.FolderPath `json:"target_folder"`
}

// folderTransferConflict is the 409 body a pre-flight refusal answers with. Error
// names which conflict it is so the frontend can tell the two apart, and a book-ID
// conflict lists every colliding ID at once so the user sees the whole set rather
// than one failed transfer at a time.
type folderTransferConflict struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	// Not omitempty: a folder conflict carries no IDs, and the client should
	// read an empty list rather than an absent field.
	ConflictingBookIDs []string `json:"conflicting_book_ids"`
}

const (
	folderTransferConflictFolder = "target_folder_conflict"
	folderTransferConflictBookID = "book_id_conflict"
)

// POST /api/shelves/{shelf_id}/folder-transfers
func (h *folderTransferHandlers) transferFolder(w http.ResponseWriter, r *http.Request) {
	sourceShelf, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	var request folderTransferRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	operation := strings.TrimSpace(request.Mode)
	if operation != task.BookTransferOperationCopy && operation != task.BookTransferOperationMove {
		http.Error(w, `mode must be "copy" or "move"`, http.StatusBadRequest)
		return
	}

	sourceFolder := append(shelf.FolderPath(nil), request.SourceFolder...)
	if len(sourceFolder) == 0 {
		http.Error(w, "source_folder is required", http.StatusBadRequest)
		return
	}
	if err := sourceShelf.ValidateFolderPath(sourceFolder); err != nil {
		h.writeErrStatus(w, r, err, "invalid source_folder", http.StatusBadRequest)
		return
	}

	targetShelfID := strings.TrimSpace(request.TargetShelf)
	if targetShelfID == "" {
		http.Error(w, "target_shelf is required", http.StatusBadRequest)
		return
	}

	// A source that is also the target belongs to the same-shelf folder endpoints,
	// and naming one shelf twice would deadlock the transfer on its own lock.
	if targetShelfID == sourceShelf.ID {
		http.Error(w, "source and target are the same shelf; move a folder within a shelf with POST .../folder-moves", http.StatusBadRequest)
		return
	}

	targetShelf, ok := h.shelves.GetShelf(targetShelfID)
	if !ok {
		http.Error(w, "target shelf not found", http.StatusNotFound)
		return
	}

	// Default to the source folder's own name so a plain "move that folder to the
	// other shelf" needs no target folder in the body, mirroring the single-book
	// endpoint's default to the source book's folder.
	targetFolder := append(shelf.FolderPath(nil), sourceFolder...)
	if request.TargetFolder != nil {
		targetFolder = append(shelf.FolderPath(nil), (*request.TargetFolder)...)
	}
	if len(targetFolder) == 0 {
		http.Error(w, "target_folder cannot be the root folder", http.StatusBadRequest)
		return
	}
	if err := targetShelf.ValidateFolderPath(targetFolder); err != nil {
		h.writeErrStatus(w, r, err, "invalid target_folder", http.StatusBadRequest)
		return
	}

	// A write to a read-only target is refused here rather than in a task the
	// caller would otherwise have to read to learn the work never happened.
	if h.rejectReadOnlyShelf(w, r, targetShelf) {
		return
	}

	// A move ends by deleting from the source, so a read-only source is refused
	// too. A copy only reads the source, so a read-only source is fine for it.
	if operation == task.BookTransferOperationMove && h.rejectReadOnlyShelf(w, r, sourceShelf) {
		return
	}

	// Force a fresh scan of both shelves before resolving anything. A plain listing
	// only schedules a refresh and answers from the cache, so a book or folder added
	// to a shared or network shelf since its last scan would be missed - the plan
	// would skip those books and the conflict checks would not see an externally
	// created folder. This mirrors the shelf-level folder-transfer preflight, which
	// forces the same scan. ErrRescanInProgress means another scan is already
	// refreshing the cache, so the following reads are as fresh as one here.
	rescanForTransfer(sourceShelf)
	rescanForTransfer(targetShelf)

	// Resolve the transfer plan from a single snapshot of the source: the folders
	// to reproduce and the books to carry. The same snapshot screens for conflicts
	// below, so the 409 check and the scheduled work see the same set.
	sourceFolders, err := sourceShelf.GetAllFolders()
	if err != nil {
		h.writeErr(w, r, err, "failed to read the source shelf")
		return
	}
	subFolders := foldersUnder(sourceFolders, sourceFolder)

	listings, err := sourceShelf.ListBooksWithCharCount()
	if err != nil {
		h.writeErr(w, r, err, "failed to read the source shelf")
		return
	}
	var books []task.FolderTransferBook
	for _, listing := range listings {
		if folderHasPrefix(listing.Folders, sourceFolder) {
			books = append(books, task.FolderTransferBook{
				ID:           listing.Book.ID(),
				SourceFolder: append(shelf.FolderPath(nil), listing.Folders...),
			})
		}
	}

	// A transfer of a folder that holds no folder and no book is a 404 now, not a
	// task that copies nothing later - the same way a missing book is a 404.
	if len(subFolders) == 0 && len(books) == 0 {
		http.Error(w, "source folder not found", http.StatusNotFound)
		return
	}

	// A folder transfer builds the destination subtree fresh, so it refuses to land
	// on a folder the target already holds rather than merging into it.
	targetFolders, err := targetShelf.GetAllFolders()
	if err != nil {
		h.writeErr(w, r, err, "failed to read the target shelf")
		return
	}
	if len(foldersUnder(targetFolders, targetFolder)) > 0 {
		h.writeJSON(w, http.StatusConflict, folderTransferConflict{
			Error:   folderTransferConflictFolder,
			Message: "the target shelf already holds a folder with this name",
		})
		return
	}

	// A move keeps every book's ID, so the target must hold none of them. Collect
	// the whole colliding set rather than stopping at the first, so the frontend
	// can list them all. A copy draws fresh IDs, so it never collides this way.
	if operation == task.BookTransferOperationMove {
		existing, err := targetBookIDs(targetShelf)
		if err != nil {
			h.writeErr(w, r, err, "failed to read the target shelf")
			return
		}
		var conflicts []string
		for _, book := range books {
			if existing[book.ID] {
				conflicts = append(conflicts, book.ID)
			}
		}
		if len(conflicts) > 0 {
			slices.Sort(conflicts)
			h.writeJSON(w, http.StatusConflict, folderTransferConflict{
				Error:              folderTransferConflictBookID,
				Message:            "the target shelf already holds books with these IDs; the move would overwrite them",
				ConflictingBookIDs: conflicts,
			})
			return
		}
	}

	h.submitTaskChain(w, r,
		task.NewFolderTransferChain(sourceShelf.ID, sourceShelf.Shelf, targetShelf.ID, targetShelf.Shelf, h.requestLogger(r), operation, sourceFolder, targetFolder, books, subFolders),
		"failed to schedule folder transfer task")
}

// foldersUnder returns every folder in all that is root itself or sits beneath it.
// An empty result means the target holds nothing under that path.
func foldersUnder(all []shelf.FolderPath, root shelf.FolderPath) []shelf.FolderPath {
	var under []shelf.FolderPath
	for _, l := range all {
		if folderHasPrefix(l, root) {
			under = append(under, append(shelf.FolderPath(nil), l...))
		}
	}
	return under
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

// rescanForTransfer forces a shelf to rebuild its book cache now, so the plan and
// the conflict checks read an authoritative listing rather than a throttled one.
// It is best-effort: a rescan already in progress is refreshing the cache anyway,
// and any other failure is left for the reads that follow to surface, since they
// answer with a real error a caller can act on.
//
// Unthrottled deliberately, and not best-effort about that one refusal: the
// rescan rate limit belongs to the button a user presses, so a transfer must
// neither spend its budget nor quietly plan from a stale cache when the budget
// is gone. See Shelf.RescanUnthrottled.
func rescanForTransfer(shelfData *shelf.ShelfData) {
	_, _ = shelfData.RescanUnthrottled()
}

// targetBookIDs is the set of book IDs the target shelf already holds, whether
// listed or sitting in its trash - the two places a move's kept ID could collide.
func targetBookIDs(target *shelf.ShelfData) (map[string]bool, error) {
	ids := map[string]bool{}

	books, err := target.ListBooks()
	if err != nil {
		return nil, err
	}
	for _, book := range books {
		ids[book.ID()] = true
	}

	trashed, err := target.ListTrashedBookIDs()
	if err != nil {
		return nil, err
	}
	for _, id := range trashed {
		ids[id] = true
	}

	return ids, nil
}
