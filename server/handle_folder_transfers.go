package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type folderTransferHandlers struct {
	*taskSubmitter
}

type folderTransferRequest struct {
	Mode         string            `json:"mode"`
	SourceFolder shelf.FolderPath  `json:"source_folder"`
	TargetShelf  string            `json:"target_shelf"`
	TargetFolder *shelf.FolderPath `json:"target_folder"`
}

type folderTransferConflict struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	ConflictingBookIDs []string `json:"conflicting_book_ids"`
}

const (
	folderTransferConflictFolder = "target_folder_conflict"
	folderTransferConflictBookID = "book_id_conflict"
)

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

	if targetShelfID == sourceShelf.ID {
		http.Error(w, "source and target are the same shelf; move a folder within a shelf with POST .../folder-moves", http.StatusBadRequest)
		return
	}

	targetShelf, ok := h.shelves.GetShelf(targetShelfID)
	if !ok {
		http.Error(w, "target shelf not found", http.StatusNotFound)
		return
	}

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

	if h.rejectReadOnlyShelf(w, r, targetShelf) {
		return
	}

	if operation == task.BookTransferOperationMove && h.rejectReadOnlyShelf(w, r, sourceShelf) {
		return
	}

	bestEffortRescanForPreflight(sourceShelf)
	bestEffortRescanForPreflight(targetShelf)

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
		if listing.Folders.HasPrefix(sourceFolder) {
			books = append(books, task.FolderTransferBook{
				ID:           listing.Book.ID(),
				SourceFolder: append(shelf.FolderPath(nil), listing.Folders...),
			})
		}
	}

	if len(subFolders) == 0 && len(books) == 0 {
		http.Error(w, "source folder not found", http.StatusNotFound)
		return
	}

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

	if h.refuseUnconfirmedReveal(w, r, sourceShelf, folderLeavingTheShelf(sourceFolder)) {
		return
	}

	h.submitTaskChain(w, r,
		task.NewFolderTransferChain(sourceShelf.ID, sourceShelf.Shelf, targetShelf.ID, targetShelf.Shelf, h.requestLogger(r), operation, sourceFolder, targetFolder, books, subFolders),
		"failed to schedule folder transfer task")
}

func foldersUnder(all []shelf.FolderPath, root shelf.FolderPath) []shelf.FolderPath {
	var under []shelf.FolderPath
	for _, l := range all {
		if l.HasPrefix(root) {
			under = append(under, slices.Clone(l))
		}
	}
	return under
}

func bestEffortRescanForPreflight(shelfData *shelf.ShelfData) {
	_, _ = shelfData.RescanUnthrottled()
}

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
