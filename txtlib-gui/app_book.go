package txtlibgui

import (
	"fmt"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
	"github.com/voilelab/plainshelf/txtlib-gui/component"
	"github.com/voilelab/plainshelf/txtlib-gui/filedialog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func (a *TxtlibApp) addBookAction() {
	filedialog.OpenFilesDialog("Select Text Files to Add", []string{"*.txt"}, func(files []fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		if len(files) == 0 {
			return
		}

		defer func() {
			for _, f := range files {
				f.Close()
			}
		}()

		for _, file := range files {
			err := a.performAddBook(file)
			if err != nil {
				dialog.ShowError(util.Errorf("failed to add book '%s': %w", file.URI().Path(), err), a.window)
			}
		}

		a.stateBar.SetText(fmt.Sprintf("Added %d book(s)", len(files)))
		a.updateBookList()
	})
}

func (a *TxtlibApp) performAddBook(file fyne.URIReadCloser) error {
	bookFilename := path.Base(file.URI().Path())
	bookTitle := strings.TrimSuffix(bookFilename, path.Ext(bookFilename))

	layers := a.layerTreeWidget.SelectedLayer()
	if layers == nil {
		layers = shelf.Layers{defaultLayer}
	}

	book, err := a.libState.lib.NewBook(layers, bookTitle)

	if err != nil {
		return util.Errorf("%w", err)
	}

	rev, err := book.NewSnapshot(file)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = book.SetCurrentSnapshot(rev.ID())
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (a *TxtlibApp) deleteBookAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to delete.", a.window)
		return
	}

	confirmDlg := dialog.NewConfirm("Delete Book", fmt.Sprintf("Are you sure you want to delete the book '%s'?", book.Title()), func(confirmed bool) {
		if confirmed {
			err := a.libState.lib.DeleteBook(book.ID())
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			a.stateBar.SetText("Deleted book: " + book.Title())
			a.updateBookList()
		}
	}, a.window)

	confirmDlg.Resize(a.scaleDialogSize(fyne.NewSize(400, 200)))
	confirmDlg.Show()
}

func (a *TxtlibApp) editBookMetaAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to edit.", a.window)
		return
	}

	editWindow := component.NewEditBookWindow(a.window, book, a.scaleDialogSize, func(updatedBook *shelf.Book) {
		a.stateBar.SetText("Updated metadata for book: " + updatedBook.Title())
		a.updateBookList()
	})
	editWindow.Show()
}

func (a *TxtlibApp) moveBookAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to move.", a.window)
		return
	}

	currentLayer := book.Layers().String()

	allLayers, err := a.libState.lib.GetAllLayers()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	if currentLayer == "" {
		selectedLayer := a.layerTreeWidget.SelectedLayer()
		if selectedLayer != nil {
			currentLayer = selectedLayer.String()
		}
	}

	joinedLayers := make([]string, len(allLayers))
	for i, layer := range allLayers {
		joinedLayers[i] = layer.String()
	}

	layerWidget := component.NewSelectLayerWidget(joinedLayers, currentLayer)

	dlg := dialog.NewCustomConfirm("Move Book", "Move", "Cancel", layerWidget.GetForm(), func(confirmed bool) {
		if !confirmed {
			return
		}

		targetLayers := layerWidget.GetSelectedLayers()
		if len(targetLayers) == 0 {
			dialog.ShowInformation("Invalid Layer", "Please enter at least one layer name.", a.window)
			return
		}

		err = a.performBookMove(book.ID(), targetLayers)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
	}, a.window)

	dlg.Resize(a.scaleDialogSize(fyne.NewSize(520, 220)))
	dlg.Show()
}

func (a *TxtlibApp) performBookMove(bookID string, targetLayers []string) error {
	if a.libState.lib == nil {
		return util.Errorf("library is not loaded")
	}

	if len(targetLayers) == 0 {
		return util.Errorf("target layer must not be empty")
	}

	book, err := a.libState.lib.GetBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	_, err = a.libState.lib.MoveBook(bookID, targetLayers)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = a.refreshLayerTree()
	if err != nil {
		return util.Errorf("%w", err)
	}

	a.updateBookListBySelectedLayer()
	a.stateBar.SetText(fmt.Sprintf("Moved book '%s' to layer: %s", book.Title(), targetLayers))
	return nil
}

func (a *TxtlibApp) handleDraggedBookDrop(bookID string, targetLayers shelf.Layers) {
	if targetLayers == nil {
		dialog.ShowInformation(
			"Invalid Target",
			"Dropping on All Layers is not supported. Choose a specific layer.", a.window)
		return
	}

	err := a.performBookMove(bookID, targetLayers)
	if err != nil {
		dialog.ShowError(err, a.window)
	}
}

func (a *TxtlibApp) openBookAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to open.", a.window)
		return
	}

	currSnapshot := book.CurrentSnapshot()
	if currSnapshot == "" {
		dialog.ShowInformation("No Content", "This book does not have any content to display.", a.window)
		return
	}

	snapshot, err := book.GetSnapshot(currSnapshot)
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	txtViewWindow := component.NewTxtViewWindow(a.fApp, snapshot, a.scaleDialogSize(fyne.NewSize(700, 500)))
	txtViewWindow.Show()
	a.stateBar.SetText("Opened book: " + book.Title())
}

func (a *TxtlibApp) openSnapshotDirectoryAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to open its snapshot directory.", a.window)
		return
	}

	snapshot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	snapshotDir := snapshot.FolderPath()
	libRoot, ok := a.requireLocalLibraryRoot("Open Snapshot Directory")
	if !ok {
		return
	}

	fullSnapshotFolderPath := path.Join(libRoot, snapshotDir)

	err = util.OpenFinder(fullSnapshotFolderPath)
	if err != nil {
		dialog.ShowError(util.Errorf("failed to open snapshot directory: %w", err), a.window)
		return
	}

	a.stateBar.SetText("Opened snapshot directory for book: " + book.Title())
}

func (a *TxtlibApp) openBookDirectoryAction() {
	book := a.getSelectedBook()
	if book == nil {
		dialog.ShowInformation("No Book Selected", "Please select a book from the list to open its directory.", a.window)
		return
	}

	bookDir := book.FolderPath()
	libRoot, ok := a.requireLocalLibraryRoot("Open Book Directory")
	if !ok {
		return
	}

	fullBookFolderPath := path.Join(libRoot, bookDir)

	err := util.OpenFinder(fullBookFolderPath)
	if err != nil {
		dialog.ShowError(util.Errorf("failed to open book directory: %w", err), a.window)
		return
	}

	a.stateBar.SetText("Opened book directory: " + book.Title())
}
