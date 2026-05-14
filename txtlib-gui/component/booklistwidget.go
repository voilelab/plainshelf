package component

import (
	"fmt"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
	"github.com/voilelab/plainshelf/txtlib-gui/guiutil"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type BookListWidget struct {
	list        *widget.List
	books       []*shelf.Book
	onSelect    func(id int, book *shelf.Book)
	onDragStart func(bookID string)
	onDragEnd   func(bookID string)
}

type draggableBookListItem struct {
	widget.BaseWidget
	root        *fyne.Container
	img         *canvas.Image
	title       *widget.Label
	desc        *widget.Label
	bookID      string
	dragging    bool
	onDragStart func(bookID string)
	onDragEnd   func(bookID string)
}

func newDraggableBookListItem(onDragStart func(bookID string), onDragEnd func(bookID string)) *draggableBookListItem {
	img := canvas.NewImageFromImage(nil)
	img.SetMinSize(fyne.NewSize(64, 64))
	title := widget.NewLabel("Book Title")
	desc := widget.NewLabel("Description")

	ret := &draggableBookListItem{
		root: container.NewHBox(
			img,
			container.NewVBox(title, desc),
		),
		img:         img,
		title:       title,
		desc:        desc,
		onDragStart: onDragStart,
		onDragEnd:   onDragEnd,
	}
	ret.ExtendBaseWidget(ret)
	return ret
}

func (i *draggableBookListItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(i.root)
}

func (i *draggableBookListItem) SetBook(book *shelf.Book) {
	i.bookID = book.ID()
	i.img.Resource = guiutil.GetBookCoverResource(book)
	i.img.Refresh()

	i.title.SetText(book.Title())
	meta := book.GetMeta()
	i.desc.SetText(fmt.Sprintf("Authors: %s | Language: %s", strings.Join(meta.Authors, ", "), meta.Language))
}

func (i *draggableBookListItem) Dragged(_ *fyne.DragEvent) {
	if i.bookID == "" || i.dragging {
		return
	}
	i.dragging = true
	if i.onDragStart != nil {
		i.onDragStart(i.bookID)
	}
}

func (i *draggableBookListItem) DragEnd() {
	if !i.dragging {
		return
	}
	i.dragging = false
	if i.onDragEnd != nil {
		i.onDragEnd(i.bookID)
	}
}

func NewBookListWidget(onSelect func(id int, book *shelf.Book), onDragStart func(bookID string), onDragEnd func(bookID string)) *BookListWidget {
	ret := &BookListWidget{onSelect: onSelect, onDragStart: onDragStart, onDragEnd: onDragEnd}

	ret.list = widget.NewList(
		func() int {
			return len(ret.books)
		},
		func() fyne.CanvasObject {
			return newDraggableBookListItem(ret.onDragStart, ret.onDragEnd)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			ret.updateListItem(id, item)
		},
	)

	ret.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(ret.books) {
			return
		}
		if ret.onSelect != nil {
			ret.onSelect(id, ret.books[id])
		}
	}

	return ret
}

func (w *BookListWidget) List() *widget.List {
	return w.list
}

func (w *BookListWidget) Select(id int) {
	if id < 0 || id >= len(w.books) {
		return
	}
	w.list.Select(id)
}

func (w *BookListWidget) Unselect() {
	w.list.UnselectAll()
}

func (w *BookListWidget) SetBooks(books []*shelf.Book) {
	w.books = books
	w.list.Refresh()
}

func (w *BookListWidget) updateListItem(id widget.ListItemID, item fyne.CanvasObject) {
	if id < 0 || id >= len(w.books) {
		return
	}

	book := w.books[id]
	itemObj := item.(*draggableBookListItem)
	itemObj.SetBook(book)
	w.list.SetItemHeight(id, 80)
}
