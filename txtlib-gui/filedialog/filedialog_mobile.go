//go:build android

package filedialog

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func openFolderDialog(title string, callback func(fyne.ListableURI, error)) {
	dlg := dialog.NewFolderOpen(callback, nil)
	dlg.SetTitleText(title)
	dlg.Show()
}

func openFilesDialog(title string, fileexts []string, callback func([]fyne.URIReadCloser, error)) {
	// Since fyne does not support multi-file selection on mobile,
	// we can reuse the openFileDialog and just return the first selected file.
	openFileDialog(title, fileexts, func(uri fyne.URIReadCloser, err error) {
		if err != nil {
			callback(nil, err)
			return
		}
		if uri == nil {
			callback(nil, nil) // User canceled
			return
		}
		callback([]fyne.URIReadCloser{uri}, nil)
	})
}

func openFileDialog(title string, fileexts []string, callback func(fyne.URIReadCloser, error)) {
	dlg := dialog.NewFileOpen(callback, nil)
	dlg.SetFilter(storage.NewExtensionFileFilter(fileexts))
	dlg.SetTitleText(title)
	dlg.Show()
}
