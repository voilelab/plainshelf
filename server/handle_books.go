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

type bookHandlers struct {
	*apiCore

	settings *settings
}

type Book struct {
	Meta   *shelf.BookMeta  `json:"meta"`
	Folder shelf.FolderPath `json:"folder"`

	CharCount int `json:"char_count,omitzero"`

	NSFWFolder *NSFWFolderRule `json:"nsfw_folder,omitzero"`

	DownloadState shelf.DownloadState `json:"download_state,omitzero"`
	DownloadedAt  *time.Time          `json:"downloaded_at,omitzero"`
}

type NSFWFolderRule struct {
	Path string `json:"path"`

	Reason string `json:"reason,omitempty"`
}

func nsfwFolderRule(shelfData *shelf.ShelfData, folder shelf.FolderPath) *NSFWFolderRule {
	rule, ok := shelfData.NSFWFolderRule(folder)
	if !ok {
		return nil
	}
	return &NSFWFolderRule{Path: rule.Path, Reason: rule.Reason}
}

func newBookResponse(shelfData *shelf.ShelfData, meta *shelf.BookMeta, folder shelf.FolderPath) Book {
	book := Book{Meta: meta, Folder: folder, NSFWFolder: nsfwFolderRule(shelfData, folder)}
	if shelfData.CanDownload() {
		state, at := shelfData.DownloadStatus(meta.ID)
		book.DownloadState = state
		if !at.IsZero() {
			book.DownloadedAt = &at
		}
	}
	return book
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

	NSFW *bool `json:"nsfw"`
}

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

func (h *bookHandlers) getBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

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

			jsonBooks[i].CharCount = b.CharCount
		}
	}

	h.writeJSON(w, http.StatusOK, jsonBooks)
}

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

	h.writeJSON(w, http.StatusCreated, newBookResponse(shelfData, newBook.GetMeta(), req.Folder))
}

type CopyBookRequest struct {
	Folder *shelf.FolderPath `json:"folder"`
}

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

	listing, ok := h.lookupVisibleBookListing(w, r, shelfData, bookID)
	if !ok {
		return
	}

	target := append(shelf.FolderPath(nil), listing.Folders...)
	if override := req.Folder; override != nil {
		target = append(shelf.FolderPath(nil), (*override)...)
	}

	copied, err := shelfData.CopyBook(bookID, target)
	if err != nil {
		h.writeErr(w, r, err, "failed to copy book")
		return
	}

	h.writeJSON(w, http.StatusCreated, newBookResponse(shelfData, copied.GetMeta(), target))
}

func (h *bookHandlers) getBook(w http.ResponseWriter, r *http.Request) {
	shelfData, listing, ok := h.loadBookListing(w, r)
	if !ok {
		return
	}

	h.writeJSON(w, http.StatusOK, newBookResponse(shelfData, listing.Book.GetMeta(), listing.Folders))
}

func (h *bookHandlers) updateBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var req UpdateBookRequest
	if !decodeStrictJSON(w, r, &req) {
		return
	}

	listing, ok := h.lookupVisibleBookListing(w, r, shelfData, bookID)
	if !ok {
		return
	}
	book := listing.Book
	folder := listing.Folders

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
		h.writeErr(w, r, err, "failed to open book source")
		return
	}
	defer src.Close()

	h.streamTextFile(w, src, "failed to write book content")
}

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
