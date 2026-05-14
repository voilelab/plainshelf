package txtlibgui

import (
	"fmt"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
	"github.com/voilelab/plainshelf/txtlib-gui/component"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const appDescription = "A simple library management tool for text-based books, built with Go and Fyne. It allows you to organize, view, and manage your collection of text files in a structured way."
const appURL = "https://github.com/voilelab/plainshelf"
const appTitle = "Plainshelf - Text Library Manager"
const appID = "com.voilelab.plainshelf"
const defaultLayer = "Uncategorized"

var appSize = fyne.NewSize(1000, 800)

// libraryState holds the currently open library and its location metadata.
type libraryState struct {
	lib       *shelf.Shelf
	uri       fyne.ListableURI
	localRoot string
}

type TxtlibApp struct {
	fApp   fyne.App
	window fyne.Window

	toolbar  *widget.Toolbar
	stateBar *widget.Label

	bookListComponent *component.BookListWidget
	bookList          []*shelf.Book
	selectedBookIdx   int
	layerTreeWidget   *component.LayerTreeWidget
	draggingBookID    string

	bookInfoWidget *component.BookInfoWidget

	isNarrowLayout  bool
	stopResizeWatch chan struct{}

	libState libraryState
}

func NewTxtlibApp() *TxtlibApp {
	fApp := app.NewWithID(appID)
	mainWindow := fApp.NewWindow(appTitle)
	mainWindow.Resize(appSize)
	mainWindow.CenterOnScreen()
	mainWindow.SetMaster()

	retApp := &TxtlibApp{
		fApp:            fApp,
		window:          mainWindow,
		selectedBookIdx: -1,
	}

	retApp.setupToolbar()
	retApp.isNarrowLayout = retApp.shouldUseNarrowLayout(mainWindow.Canvas().Size().Width)
	retApp.setup()
	retApp.setupMenu()
	retApp.startResizeWatcher()
	mainWindow.SetOnClosed(retApp.stopResizeWatcher)

	return retApp
}

func (a *TxtlibApp) setupMenu() {
	menu := fyne.NewMainMenu(
		fyne.NewMenu("Library",
			&fyne.MenuItem{
				Label:  "Open Library (Local)",
				Icon:   theme.FolderOpenIcon(),
				Action: a.openLibraryAction,
			},
			&fyne.MenuItem{
				Label:  "Open Library (WebDAV)",
				Icon:   theme.FolderOpenIcon(),
				Action: a.openLibraryWebDAVAction,
			},
		),
		fyne.NewMenu("Book",
			&fyne.MenuItem{
				Label:  "Add Book",
				Icon:   theme.ContentAddIcon(),
				Action: a.addBookAction,
			},
			&fyne.MenuItem{
				Label:  "Move Book",
				Icon:   theme.FolderIcon(),
				Action: a.moveBookAction,
			},
			&fyne.MenuItem{
				Label:  "Edit Book Metadata",
				Icon:   theme.DocumentCreateIcon(),
				Action: a.editBookMetaAction,
			},
			&fyne.MenuItem{
				Label:  "Delete Book",
				Icon:   theme.DeleteIcon(),
				Action: a.deleteBookAction,
			},
			&fyne.MenuItem{
				Label:  "Open Book",
				Icon:   theme.DocumentIcon(),
				Action: a.openBookAction,
			},
		),
		fyne.NewMenu("Help",
			&fyne.MenuItem{
				Label: "About",
				Action: func() {
					dialog.ShowInformation("About Txtlib", fmt.Sprintf("%s\n\n%s\n\n%s", appTitle, appDescription, appURL), a.window)
				},
			},
		),
	)

	a.window.SetMainMenu(menu)
}

func (a *TxtlibApp) setup() {
	stateBar := widget.NewLabel("Ready")
	a.stateBar = stateBar
	a.layerTreeWidget = component.NewLayerTreeWidget(func(_ string) {
		if a.bookListComponent == nil {
			return
		}
		a.updateBookListBySelectedLayer()
	}, func(_ string) {})

	a.bookListComponent = component.NewBookListWidget(func(id int, book *shelf.Book) {
		a.selectedBookIdx = id
		a.stateBar.SetText("Selected book: " + book.Title())
		a.bookInfoWidget.SetBook(book)
	}, func(bookID string) {
		a.draggingBookID = bookID
		a.layerTreeWidget.SetDragActive(true)
		a.stateBar.SetText("Dragging book: drop onto a layer")
	}, func(bookID string) {
		targetLayer := a.layerTreeWidget.HoveredLayer()
		a.layerTreeWidget.SetDragActive(false)
		a.draggingBookID = ""

		if targetLayer == nil {
			return
		}

		a.handleDraggedBookDrop(bookID, targetLayer)
	})

	a.bookInfoWidget = component.NewBookInfoWidget(
		a.openBookAction, a.openBookDirectoryAction,
		a.openSnapshotDirectoryAction, a.editBookMetaAction,
		a.moveBookAction)
	a.window.SetContent(a.buildContent())
}

func (a *TxtlibApp) setupToolbar() {
	a.toolbar = widget.NewToolbar(
		widget.NewToolbarAction(theme.FolderOpenIcon(), func() {
			a.openLibraryAction()
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.ContentAddIcon(), func() {
			a.addBookAction()
		}),
		widget.NewToolbarAction(theme.DeleteIcon(), func() {
			a.deleteBookAction()
		}),
	)
}

func (a *TxtlibApp) updateBookList() {
	err := a.refreshLayerTree()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.updateBookListBySelectedLayer()
}

func (a *TxtlibApp) getSelectedBook() *shelf.Book {
	if a.selectedBookIdx < 0 || a.selectedBookIdx >= len(a.bookList) {
		return nil
	}
	return a.bookList[a.selectedBookIdx]
}

func (a *TxtlibApp) refreshLayerTree() error {
	if a.libState.lib == nil {
		a.layerTreeWidget.SetLayers(nil)
		return nil
	}

	allLayers, err := a.libState.lib.GetAllLayers()
	if err != nil {
		return util.Errorf("%w", err)
	}

	a.layerTreeWidget.SetLayers(allLayers)
	return nil
}

func (a *TxtlibApp) updateBookListBySelectedLayer() {
	if a.libState.lib == nil {
		a.bookList = nil
		a.selectedBookIdx = -1
		a.bookListComponent.SetBooks(nil)
		return
	}

	var (
		books []*shelf.Book
		err   error
	)

	selectedLayer := a.layerTreeWidget.SelectedLayer()
	if selectedLayer == nil {
		books, err = a.libState.lib.ListBooks()
	} else {
		books, err = a.libState.lib.GetBooksByLayer(selectedLayer)
	}

	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	a.bookList = books
	a.selectedBookIdx = -1
	a.bookListComponent.SetBooks(books)
	a.bookListComponent.Unselect()

	if selectedLayer == nil {
		a.stateBar.SetText(fmt.Sprintf("Showing all layers (%d books)", len(books)))
	} else {
		a.stateBar.SetText(fmt.Sprintf("Showing layer: %s (%d books)", selectedLayer.String(), len(books)))
	}
}

func (a *TxtlibApp) Run() {
	a.loadLastLibrary()
	a.window.ShowAndRun()
}
