package server

import (
	"encoding/json/v2"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
)

// folderHandlers serves the folder tree the books are filed under.
type folderHandlers struct {
	*apiCore
}

// GET /api/shelves/{shelf_id}/folders
func (h *folderHandlers) getFolders(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folders, err := shelfData.GetAllFolders()
	if err != nil {
		h.writeErr(w, r, err, "failed to get folders")
		return
	}

	// The tree this returns is what the breadcrumbs and the move-book
	// destination menu are built from, so a folder left in it after its books
	// were filtered out would name what the filter is hiding.
	folders, err = h.visibility(shelfData).filterFolders(folders)
	if err != nil {
		h.writeErr(w, r, err, "failed to get folders")
		return
	}

	h.writeJSON(w, http.StatusOK, folders)
}

// POST /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) createFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	parent := append(shelf.FolderPath(nil), folderParts[:len(folderParts)-1]...)
	name := folderParts[len(folderParts)-1]
	if err := shelfData.NewFolder(parent, name); err != nil {
		h.writeErr(w, r, err, "failed to create folder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// nsfwRevealConflict is the 409 body a folder route answers when the change
// would take content this request cannot see out of a marked subtree - a folder
// rule in shelf.json marks a path, so moving the folder out from under the path
// unmarks everything below it at once, and show_nsfw is what was hiding it.
//
// It is a refusal the caller can retry rather than a rule: PSW-118 chose to let
// the move happen and make the user say so, because shelf.json is the user's own
// file and rearranging a shelf is not an operation PlainShelf should forbid.
type nsfwRevealConflict struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	// HiddenBooks is how many books this request cannot currently see would be
	// served once the change is made. It is 0 when the whole disclosure is a
	// folder name, so a client must not read 0 as "nothing would change".
	HiddenBooks int `json:"hidden_books"`
}

// nsfwRevealConflictKind is the stable identifier of that refusal, in the same
// position as the folder-transfer conflict kinds so one client branch reads
// both.
const nsfwRevealConflictKind = "nsfw_reveal_requires_confirmation"

// confirmedReveal reports whether the caller has been told what the change
// reveals and asked for it anyway.
//
// A query parameter rather than a body field because the three routes that can
// refuse this way carry three different bodies, and the rename's has room for a
// folder name and nothing else. It is spelled the way the confirmed similarity
// scan spells its own - ?confirm=1 - so a client learns the convention once.
func confirmedReveal(r *http.Request) bool {
	return r.URL.Query().Get("confirm") == "1"
}

// validFolderPaths answers the request error a malformed path earns and reports
// false, so a caller can put that answer ahead of a refusal that would otherwise
// hide it. The shelf validates these paths again when it acts on them; this only
// settles the order the caller answers in.
func (c *apiCore) validFolderPaths(
	w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, paths ...shelf.FolderPath,
) bool {
	for _, path := range paths {
		if err := shelfData.ValidateFolderPath(path); err != nil {
			c.writeErrStatus(w, r, err, "invalid folder name", http.StatusBadRequest)
			return false
		}
	}
	return true
}

// refreshBeforeReveal forces the fresh scan the check below needs, on the shelves
// where a mark makes one necessary.
//
// The listings it reads are answered from a throttled cache, so a marked folder
// another writer added to a shared or network shelf since the last scan would be
// missing from them - while the move that follows works on the real filesystem
// and would carry that folder out from under its mark. This is the same forced
// scan, for the same reason, that the cross-shelf transfer runs before resolving
// its plan; that endpoint has already run its own by the time it asks, so it
// does not call this.
func (c *apiCore) refreshBeforeReveal(r *http.Request, shelfData *shelf.ShelfData) {
	if confirmedReveal(r) || !c.visibility(shelfData).couldReveal() {
		return
	}
	rescanForPreflight(shelfData)
}

// refuseUnconfirmedReveal answers 409 and reports true when change would unhide
// something the caller has not confirmed. A caller that gets false may go ahead.
func (c *apiCore) refuseUnconfirmedReveal(
	w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, change folderChange,
) bool {
	if confirmedReveal(r) {
		return false
	}

	reveal, err := c.visibility(shelfData).revealedBy(change)
	if err != nil {
		c.writeErr(w, r, err, "failed to read the shelf")
		return true
	}
	if !reveal.any() {
		return false
	}

	c.writeJSON(w, http.StatusConflict, nsfwRevealConflict{
		Error: nsfwRevealConflictKind,
		Message: "this folder holds content the shelf marks as adult and the show_nsfw setting is hiding; " +
			"the change would take it out of the marked folder and make it visible. Retry with confirm=1 to go ahead.",
		HiddenBooks: reveal.Books,
	})
	return true
}

type renameFolderRequest struct {
	Name string `json:"name"`
}

// PATCH /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) renameFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil || len(folderParts) == 0 || strings.Join(folderParts, "") == "" {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	var req renameFolderRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" || strings.Contains(newName, "/") {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	// A rename is a move that keeps the parent, so it unmarks a subtree the same
	// way: Fiction/Adult renamed to Fiction/General matches no rule any more.
	renamed := append(append(shelf.FolderPath(nil), folderParts[:len(folderParts)-1]...), newName)

	// Both paths are validated here rather than left to RenameFolder, because
	// the two refusals below have to come after it: a malformed path is a
	// request error whatever state the shelf is in, and the same order is what
	// the cross-shelf transfer already runs. RenameFolder validates them again
	// under its own lock, which is where the answer that counts comes from.
	if !h.validFolderPaths(w, r, shelfData, folderParts, renamed) {
		return
	}

	// The shelf refuses every write to a read-only shelf, so this rename has
	// nothing to disclose - and the disclosure below is a question put to the
	// user, which must not run ahead of a refusal.
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.refreshBeforeReveal(r, shelfData)
	if h.refuseUnconfirmedReveal(w, r, shelfData, folderMovedTo(folderParts, renamed)) {
		return
	}

	if err := shelfData.RenameFolder(folderParts, newName); err != nil {
		h.writeErrStatus(w, r, err, "failed to rename folder", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type moveFolderRequest struct {
	Folder       shelf.FolderPath `json:"folder"`
	TargetFolder shelf.FolderPath `json:"target_folder"`
}

// POST /api/shelves/{shelf_id}/folder-moves
func (h *folderHandlers) moveFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	var req moveFolderRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Folder) == 0 {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	// MoveFolder keeps the folder's own name and hangs it under the target, so
	// that is the path every folder below it is rebased onto.
	moved := append(append(shelf.FolderPath(nil), req.TargetFolder...), req.Folder[len(req.Folder)-1])

	// Validated ahead of both refusals below, for the reason renameFolder gives.
	// The target parent is checked rather than the assembled path, which is what
	// MoveFolder itself validates.
	if !h.validFolderPaths(w, r, shelfData, req.Folder, req.TargetFolder) {
		return
	}

	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.refreshBeforeReveal(r, shelfData)
	if h.refuseUnconfirmedReveal(w, r, shelfData, folderMovedTo(req.Folder, moved)) {
		return
	}

	if err := shelfData.MoveFolder(req.Folder, req.TargetFolder); err != nil {
		h.writeErrStatus(w, r, err, "failed to move folder", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) deleteFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	if err := shelfData.DeleteFolder(folderParts); err != nil {
		h.writeErr(w, r, err, "failed to delete folder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
