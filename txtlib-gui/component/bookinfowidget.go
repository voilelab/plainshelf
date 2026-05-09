package component

import (
	"fmt"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/txtlib"
	"github.com/voilelab/plainshelf/txtlib-gui/guiutil"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type BookInfoWidget struct {
	container *fyne.Container
	img       *canvas.Image
	title     *widget.Label
	desc      *widget.Label
}

func NewBookInfoWidget(openBookAction, openBookDirectoryAction, openSnapshotDirectoryAction, editBookMetaAction, moveBookAction func()) *BookInfoWidget {
	bookTitle := widget.NewLabel("No library loaded")
	bookTitle.TextStyle = fyne.TextStyle{Bold: true}
	bookTitle.Wrapping = fyne.TextWrapWord

	bookDescContent := widget.NewLabel("Select a book to see details")
	bookDescContent.Wrapping = fyne.TextWrapWord

	bookImg := canvas.NewImageFromResource(theme.DocumentIcon())
	bookImg.SetMinSize(fyne.NewSize(128, 128))

	w := &BookInfoWidget{
		img:   bookImg,
		title: bookTitle,
		desc:  bookDescContent,
	}

	w.container = container.NewVBox(
		w.img,
		w.title,
		w.desc,
		widget.NewButtonWithIcon("Open Book", theme.DocumentIcon(), openBookAction),
		widget.NewButtonWithIcon("Open Book Directory", theme.FolderIcon(), openBookDirectoryAction),
		widget.NewButtonWithIcon("Open Snapshot Directory", theme.FolderIcon(), openSnapshotDirectoryAction),
		widget.NewButtonWithIcon("Edit Metadata", theme.DocumentCreateIcon(), editBookMetaAction),
		widget.NewButtonWithIcon("Move Book", theme.FolderIcon(), moveBookAction),
	)

	return w
}

func (w *BookInfoWidget) Container() *fyne.Container {
	return w.container
}

func (w *BookInfoWidget) SetBook(book *txtlib.Book) {
	meta := book.GetMeta()

	w.img.Resource = guiutil.GetBookCoverResource(book)
	w.img.Refresh()

	w.title.SetText(book.Title())
	desc := fmt.Sprintf("Authors: %s\nLanguage: %s", strings.Join(meta.Authors, ", "), meta.Language)
	if !meta.PublishedAt.IsZero() {
		desc += "\nPublished: " + time.Time(meta.PublishedAt).Format("2006-01-02")
	}
	if strings.TrimSpace(meta.Comments) != "" {
		desc += "\n\nComments:\n" + meta.Comments
	}
	w.desc.SetText(desc)
	w.container.Refresh()
}
