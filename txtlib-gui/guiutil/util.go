package guiutil

import (
	"log"
	"path"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/voilelab/plainshelf/shelf"
)

func GetBookCoverResource(book *shelf.Book) fyne.Resource {
	coverPath := book.GetMeta().Cover
	if coverPath == "" {
		return theme.DocumentIcon()
	}

	data, _, err := book.OpenCover()
	if err != nil {
		log.Println("Error loading cover for book", book.Title(), ":", err)
		return theme.DocumentIcon()
	}

	return fyne.NewStaticResource(path.Base(coverPath), data)
}
