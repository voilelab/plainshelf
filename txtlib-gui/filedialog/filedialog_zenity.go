//go:build !android

package filedialog

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/ncruces/zenity"
	"github.com/voilelab/plainshelf/internal/util"
)

func openFolderDialog(title string, callback func(fyne.ListableURI, error)) {
	func() {
		folderPath, err := zenity.SelectFile(
			zenity.Title(title),
			zenity.Directory(),
		)

		if err == zenity.ErrCanceled {
			callback(nil, nil)
			return
		}

		if err != nil {
			callback(nil, util.Errorf("%w", err))
			return
		}

		uri, err := storage.ParseURI("file://" + folderPath)
		if err != nil {
			callback(nil, util.Errorf("invalid folder path: %w", err))
			return
		}

		listableURI, err := storage.ListerForURI(uri)
		if err != nil {
			callback(nil, util.Errorf("cannot list folder: %w", err))
			return
		}

		callback(listableURI, nil)
	}()
}

func openFilesDialog(title string, fileexts []string, callback func([]fyne.URIReadCloser, error)) {
	func() {
		filePaths, err := zenity.SelectFileMultiple(
			zenity.Title(title),
			zenity.FileFilter{
				Name:     "Supported Files",
				Patterns: fileexts,
			},
		)

		if err == zenity.ErrCanceled {
			callback(nil, nil)
			return
		}

		if err != nil {
			callback(nil, util.Errorf("%w", err))
			return
		}

		var readClosers []fyne.URIReadCloser
		fail := func() {
			for _, rc := range readClosers {
				rc.Close()
			}
		}

		for _, filePath := range filePaths {
			uri, err := storage.ParseURI("file://" + filePath) // Ensure the path is valid
			if err != nil {
				fail()
				callback(nil, util.Errorf("invalid file path: %w", err))
				return
			}

			readCloser, err := storage.Reader(uri)
			if err != nil {
				fail()
				callback(nil, util.Errorf("cannot open file: %w", err))
				return
			}

			readClosers = append(readClosers, readCloser)
		}

		callback(readClosers, nil)
	}()
}

func openFileDialog(title string, fileexts []string, callback func(fyne.URIReadCloser, error)) {
	func() {
		filePath, err := zenity.SelectFile(
			zenity.Title(title),
			zenity.FileFilter{
				Name:     "Supported Files",
				Patterns: fileexts,
			},
		)

		if err == zenity.ErrCanceled {
			callback(nil, nil)
			return
		}

		if err != nil {
			callback(nil, util.Errorf("%w", err))
			return
		}

		uri, err := storage.ParseURI("file://" + filePath)
		if err != nil {
			callback(nil, util.Errorf("invalid file path: %w", err))
			return
		}

		readCloser, err := storage.Reader(uri)
		if err != nil {
			callback(nil, util.Errorf("cannot open file: %w", err))
			return
		}

		callback(readCloser, nil)
	}()
}
