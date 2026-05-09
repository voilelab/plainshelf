package filedialog

import "fyne.io/fyne/v2"

// OpenFolderDialog opens a folder selection dialog and returns the selected folder path.
// if path is empty and error is nil, it means the user canceled the dialog.
func OpenFolderDialog(title string, callback func(fyne.ListableURI, error)) {
	openFolderDialog(title, callback)
}

// OpenFilesDialog opens a file selection dialog and returns the selected file paths.
// if paths is empty and error is nil, it means the user canceled the dialog.
func OpenFilesDialog(title string, fileexts []string, callback func([]fyne.URIReadCloser, error)) {
	openFilesDialog(title, fileexts, callback)
}

// OpenFileDialog opens a file selection dialog and returns the selected file path.
// if path is empty and error is nil, it means the user canceled the dialog.
func OpenFileDialog(title string, fileexts []string, callback func(fyne.URIReadCloser, error)) {
	openFileDialog(title, fileexts, callback)
}
