package shelf

// The shelf's answer to "is this book adult content", assembled from the two
// places the answer can be written down: content.nsfw_folders in shelf.json,
// which marks a folder subtree, and nsfw in a book's own book.json, which marks
// one book. Both travel with the shelf, so every reader of it - this server, the
// desktop app, the Android client reading it from pCloud - computes the same
// answer without a central list to keep in sync.
//
// Nothing here hides anything. This file only decides what is marked; who acts
// on the mark is a separate question.

// IsBookNSFW reports whether the book at this folder path, carrying this
// metadata, is adult content.
//
// The two sources add, they do not override. A folder rule marks every book
// below it, and a book's own nsfw marks that one book — but nsfw: false on a
// book does NOT take it out of a marked folder. That asymmetry is deliberate and
// conservative: the failure it rules out is a book that should have been marked
// and quietly was not, which is the failure that matters here. Taking one book
// out of a marked folder is done by moving it out of that folder.
//
// meta may be nil, which is read as a book that says nothing about itself.
func (s *Shelf) IsBookNSFW(folders FolderPath, meta *BookMeta) bool {
	if meta != nil && meta.NSFW {
		return true
	}
	return s.nsfw.IsNSFWFolder(folders)
}

// NSFWFolderReason reports why a folder path is marked as adult content, and
// whether it is marked at all. The reason is the one written in shelf.json, so
// it is a phrase the user wrote about their own shelf; it may be empty for a
// folder listed without one.
func (s *Shelf) NSFWFolderReason(folders FolderPath) (reason string, marked bool) {
	return s.nsfw.MatchNSFWFolder(folders)
}
