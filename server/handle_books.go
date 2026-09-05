package server

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// bookHandlers serves the book routes. It reads settings because the cover
// upload path has to know whether this installation stores covers as JPEG.
type bookHandlers struct {
	*apiCore

	settings *settings
}

type Book struct {
	Meta   *shelf.BookMeta  `json:"meta"`
	Folder shelf.FolderPath `json:"folder"`

	// CharCount is only populated when the request includes
	// include=char_count; it is omitted otherwise, so the default response
	// shape is unchanged.
	CharCount int `json:"char_count,omitzero"`

	// NSFWFolder is the shelf.json rule marking this book's folder, absent when
	// none reaches it.
	//
	// Meta.NSFW is the book's own half of the mark and a client may write it
	// back; this half belongs to the shelf and it may not. A client that offered
	// one checkbox for both would let a user clear a mark that then reappears on
	// the next read, so the two halves are reported separately - and the whole
	// answer, the one the filter acts on, is Meta.NSFW or this being present.
	NSFWFolder *NSFWFolderRule `json:"nsfw_folder,omitzero"`
}

// NSFWFolderRule is one content.nsfw_folders entry, as written in shelf.json.
type NSFWFolderRule struct {
	Path string `json:"path"`

	// Reason is what the person who wrote the entry noted, and is often absent;
	// a client with nothing here names the path instead.
	Reason string `json:"reason,omitempty"`
}

// nsfwFolderRule is the rule marking this folder, or nil. Every Book response
// goes through it so no route can report a book as unmarked that another
// reports as marked.
func nsfwFolderRule(shelfData *shelf.ShelfData, folder shelf.FolderPath) *NSFWFolderRule {
	rule, ok := shelfData.NSFWFolderRule(folder)
	if !ok {
		return nil
	}
	return &NSFWFolderRule{Path: rule.Path, Reason: rule.Reason}
}

// newBookResponse assembles the response for one book sitting at folder.
func newBookResponse(shelfData *shelf.ShelfData, meta *shelf.BookMeta, folder shelf.FolderPath) Book {
	return Book{Meta: meta, Folder: folder, NSFWFolder: nsfwFolderRule(shelfData, folder)}
}

type UpdateBookRequest struct {
	Title       *string            `json:"title"`
	Authors     *[]string          `json:"authors"`
	Tags        *[]string          `json:"tags"`
	Identifiers *map[string]string `json:"identifiers"`
	Language    *string            `json:"language"`
	Comment     *string            `json:"comment"`
	Star        *int               `json:"star"`
	Format      *string            `json:"format"`
	PublishedAt *util.JSONDate     `json:"published_at"`
	Folder      *shelf.FolderPath  `json:"folder"`

	// NSFW writes the book's own half of the adult-content mark. It cannot
	// clear a mark the book's folder carries: shelf.json decides that one, and
	// Shelf.IsBookNSFW adds the two rather than letting either override.
	NSFW *bool `json:"nsfw"`
}

// folderPath locates a book on disk for the desktop client's "show in file
// manager" action. It is not an HTTP route.
func (h *bookHandlers) folderPath(shelfID, bookID string) (string, error) {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return "", util.Errorf("shelf ID cannot be empty")
	}

	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return "", util.Errorf("book ID cannot be empty")
	}

	shelfData, ok := h.shelves.GetShelf(shelfID)
	if !ok {
		return "", util.Errorf("shelf with ID %q not found", shelfID)
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return book.PackagePath(), nil
}

// GET /api/shelves/{shelf_id}/books
func (h *bookHandlers) getBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	// The listing carries the character counts whether or not this request
	// asked for them: they come out of the book cache, so fetching them costs
	// nothing beyond the listing itself.
	books, err := h.visibility(shelfData).listBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	includeCharCount := false
	for inc := range strings.SplitSeq(r.URL.Query().Get("include"), ",") {
		if strings.TrimSpace(inc) == "char_count" {
			includeCharCount = true
			break
		}
	}

	jsonBooks := make([]Book, len(books))
	for i, b := range books {
		jsonBooks[i] = newBookResponse(shelfData, b.Book.GetMeta(), b.Folders)
		if includeCharCount {
			// A book with a broken or missing source reports 0, which omitzero
			// drops: one damaged book must not fail the whole listing.
			jsonBooks[i].CharCount = b.CharCount
		}
	}

	h.writeJSON(w, http.StatusOK, jsonBooks)
}

// POST /api/shelves/{shelf_id}/books
func (h *bookHandlers) createBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	var req struct {
		Title  string           `json:"title"`
		Folder shelf.FolderPath `json:"folder"`
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		h.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(bs, &req)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// The empty source and the current-source pointer are written while the book
	// is still staged, so a failure here leaves no half-built book on disk.
	newBook, err := shelfData.NewBookWith(req.Folder, req.Title, func(book *shelf.Book) error {
		source, err := book.NewSource(nil)
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		h.writeErr(w, r, err, "failed to create new book")
		return
	}

	// The book was created under req.Folder, so that is where it now sits; the
	// book itself does not carry its folder back.
	h.writeJSON(w, http.StatusCreated, newBookResponse(shelfData, newBook.GetMeta(), req.Folder))
}

// CopyBookRequest carries the optional destination for a copy. When the field is
// unset - including an empty request body - the copy lands in the source book's
// own folder.
type CopyBookRequest struct {
	Folder *shelf.FolderPath `json:"folder"`
}

// POST /api/shelves/{shelf_id}/books/{book_id}/copies
func (h *bookHandlers) copyBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var req CopyBookRequest
	if !decodeOptionalStrictJSON(w, r, &req) {
		return
	}

	listing, ok := h.lookupBookListing(w, r, shelfData, bookID)
	if !ok {
		return
	}

	// Default to the source book's own folder so a plain "duplicate" needs no body.
	target := append(shelf.FolderPath(nil), listing.Folders...)
	if override := req.Folder; override != nil {
		target = append(shelf.FolderPath(nil), (*override)...)
	}

	copied, err := shelfData.CopyBook(bookID, target)
	if err != nil {
		h.writeErr(w, r, err, "failed to copy book")
		return
	}

	// The copy landed under target, so that is its folder.
	h.writeJSON(w, http.StatusCreated, newBookResponse(shelfData, copied.GetMeta(), target))
}

// GET /api/shelves/{shelf_id}/books/{book_id}
func (h *bookHandlers) getBook(w http.ResponseWriter, r *http.Request) {
	shelfData, listing, ok := h.loadBookListing(w, r)
	if !ok {
		return
	}

	h.writeJSON(w, http.StatusOK, newBookResponse(shelfData, listing.Book.GetMeta(), listing.Folders))
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}
func (h *bookHandlers) updateBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	// The body is decoded before the book is loaded so a malformed body is
	// still reported as 400 for a book that does not exist.
	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var req UpdateBookRequest
	if !decodeStrictJSON(w, r, &req) {
		return
	}

	listing, ok := h.lookupBookListing(w, r, shelfData, bookID)
	if !ok {
		return
	}
	book := listing.Book
	folder := listing.Folders

	// Refuse a book this build must not modify before doing anything, otherwise
	// a folder move would be applied to disk and then reported as a failure.
	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, r, err, "failed to update book metadata")
		return
	}

	if req.Folder != nil {
		moveTo := append(shelf.FolderPath(nil), (*req.Folder)...)
		movedBook, err := shelfData.MoveBook(bookID, moveTo)
		if err != nil {
			h.writeErr(w, r, err, "failed to move book folder")
			return
		}
		// The book now sits where it was moved to.
		book = movedBook
		folder = moveTo
	}

	meta := *book.GetMeta()
	applyBookPatch(&meta, &req)

	if err := book.SetMeta(&meta); err != nil {
		h.writeErr(w, r, err, "failed to update book metadata")
		return
	}

	h.writeJSON(w, http.StatusOK, newBookResponse(shelfData, &meta, folder))
}

// applyBookPatch validates nothing: the field rules belong to shelf, which
// enforces them in SetMeta before anything reaches disk.
func applyBookPatch(meta *shelf.BookMeta, req *UpdateBookRequest) {
	if req.Title != nil {
		meta.Title = *req.Title
	}
	if req.Authors != nil {
		meta.Authors = append([]string(nil), (*req.Authors)...)
	}
	if req.Tags != nil {
		meta.Tags = append([]string(nil), (*req.Tags)...)
	}
	if req.Identifiers != nil {
		meta.Identifiers = *req.Identifiers
	}
	if req.Language != nil {
		meta.Language = *req.Language
	}
	if req.Comment != nil {
		meta.Comments = *req.Comment
	}
	if req.PublishedAt != nil {
		meta.PublishedAt = *req.PublishedAt
	}
	if req.Star != nil {
		meta.Star = *req.Star
	}
	if req.Format != nil {
		meta.Format = *req.Format
	}
	if req.NSFW != nil {
		meta.NSFW = *req.NSFW
	}

	meta.UpdatedAt = util.JSONTime(time.Now())
}

// GET /api/shelves/{shelf_id}/books/{book_id}/content
func (h *bookHandlers) getBookContent(w http.ResponseWriter, r *http.Request) {
	_, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	source, err := book.ResolveCurrentSource()
	if err != nil {
		h.writeErr(w, r, err, "failed to get book source")
		return
	}

	src, err := source.Open()
	if err != nil {
		h.Error("failed to open book source", "error", err)
		http.Error(w, "failed to open book source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	h.streamTextFile(w, src, "failed to write book content")
}

// GET /api/shelves/{shelf_id}/books/duplicate
func (h *bookHandlers) findDuplicateBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	md5Groups := map[string][]string{}
	books, err := h.visibility(shelfData).listBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	for _, listing := range books {
		b := listing.Book
		source, err := b.GetSource(b.CurrentSource())
		if err != nil {
			h.Warn("failed to get source for book", "book_id", b.ID(), "error", err)
			continue
		}
		meta := source.GetMeta()
		md5Groups[meta.MD5Hash] = append(md5Groups[meta.MD5Hash], b.ID())
	}

	groups := [][]string{}
	for _, ids := range md5Groups {
		if len(ids) > 1 {
			groups = append(groups, ids)
		}
	}

	h.writeJSON(w, http.StatusOK, groups)
}
