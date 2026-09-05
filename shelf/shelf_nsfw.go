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
func (s *Shelf) IsBookNSFW(folders FolderPath, meta *BookMeta) bool {
	if meta != nil && meta.NSFW {
		return true
	}
	return s.IsNSFWFolder(folders)
}

// IsNSFWFolder reports whether this folder path lies in a marked subtree,
// without asking about any book in it.
//
// A caller that lists folders needs the question in this form: a marked folder
// is marked whether or not it currently holds a book, so there is no book to
// ask IsBookNSFW about.
func (s *Shelf) IsNSFWFolder(folders FolderPath) bool {
	return s.nsfw.IsNSFWFolder(folders)
}
