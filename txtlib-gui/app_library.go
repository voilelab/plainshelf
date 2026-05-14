package txtlibgui

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
	"github.com/voilelab/plainshelf/txtlib-gui/filedialog"
	"github.com/voilelab/plainshelf/txtlib-gui/guiutil"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const preferenceKeyLastLibraryConf = "lastLibraryConf"

func (a *TxtlibApp) loadLastLibrary() {
	conf, err := a.loadSavedLibraryConf()
	if err != nil {
		log.Printf("Failed to restore saved library config: %v\n", err)
		return
	}

	if conf == nil {
		return
	}

	lib, err := guiutil.NewLib(conf)
	if err != nil {
		// Failed to open library, log but don't show error dialog
		log.Printf("Failed to auto-load last library: %v\n", err)
		return
	}

	a.setLibraryByConf(lib, conf)
	if a.libState.uri != nil {
		a.stateBar.SetText("Library loaded: " + guiutil.DisplayURI(a.libState.uri))
	} else {
		a.stateBar.SetText("Library loaded")
	}
	a.updateBookList()
}

func (a *TxtlibApp) openLibraryAction() {
	filedialog.OpenFolderDialog("Select Library Folder", func(folder fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		if folder == nil {
			return
		}

		conf := &guiutil.LibConf{
			Type: guiutil.LibTypeURI,
			Conf: guiutil.LibConfURI{URI: folder.String()},
		}

		lib, err := guiutil.NewLib(conf)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		a.setLibraryByConf(lib, conf)
		a.stateBar.SetText("Library opened: " + guiutil.DisplayURI(folder))
		a.updateBookList()
	})
}

// webDAVFormFields holds the entry widgets for the WebDAV connection form.
type webDAVFormFields struct {
	host     *widget.Entry
	port     *widget.Entry
	user     *widget.Entry
	password *widget.Entry
	baseDir  *widget.Entry
}

// buildWebDAVForm constructs the WebDAV connection form and returns the widget
// and a struct of the individual entry fields for reading submitted values.
func buildWebDAVForm() (*widget.Form, *webDAVFormFields) {
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("https://example.com or example.com")

	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("Optional, e.g. 443")

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	baseDirEntry := widget.NewEntry()
	baseDirEntry.SetText("/")
	baseDirEntry.SetPlaceHolder("/ or /path/to/library")

	form := widget.NewForm(
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("User", userEntry),
		widget.NewFormItem("Password", passwordEntry),
		widget.NewFormItem("BaseDir", baseDirEntry),
	)

	return form, &webDAVFormFields{
		host:     hostEntry,
		port:     portEntry,
		user:     userEntry,
		password: passwordEntry,
		baseDir:  baseDirEntry,
	}
}

func (a *TxtlibApp) openLibraryWebDAVAction() {
	form, fields := buildWebDAVForm()

	dlg := dialog.NewCustomConfirm("Open Library (WebDAV)", "Open", "Cancel", form, func(confirmed bool) {
		if !confirmed {
			return
		}

		host := strings.TrimSpace(fields.host.Text)
		if host == "" {
			dialog.ShowInformation("Missing Host", "Please enter a WebDAV host URL.", a.window)
			return
		}

		port := 0
		if strings.TrimSpace(fields.port.Text) != "" {
			parsedPort, err := strconv.Atoi(strings.TrimSpace(fields.port.Text))
			if err != nil || parsedPort <= 0 {
				dialog.ShowInformation("Invalid Port", "Port must be a positive integer.", a.window)
				return
			}
			port = parsedPort
		}

		conf := &guiutil.LibConf{
			Type: guiutil.LibTypeWebDAV,
			Conf: guiutil.LibConfWebDAV{
				Host:     host,
				Port:     port,
				User:     strings.TrimSpace(fields.user.Text),
				Password: fields.password.Text,
				BaseDir:  strings.TrimSpace(fields.baseDir.Text),
			},
		}

		lib, err := guiutil.NewLib(conf)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		a.setLibraryByConf(lib, conf)
		a.stateBar.SetText("Library opened (WebDAV): " + host)
		a.updateBookList()
	}, a.window)

	dlg.Resize(a.scaleDialogSize(fyne.NewSize(560, 320)))
	dlg.Show()
}

func (a *TxtlibApp) loadSavedLibraryConf() (*guiutil.LibConf, error) {
	prefs := a.fApp.Preferences()
	storedConf := prefs.String(preferenceKeyLastLibraryConf)
	if storedConf == "" {
		return nil, nil
	}

	var conf guiutil.LibConf
	if err := json.Unmarshal([]byte(storedConf), &conf); err != nil {
		prefs.SetString(preferenceKeyLastLibraryConf, "")
		return nil, util.Errorf("failed to parse saved library config: %w", err)
	}

	return &conf, nil
}

func (a *TxtlibApp) setLibraryByConf(lib *shelf.Lib, conf *guiutil.LibConf) {
	state := libraryState{lib: lib}

	if conf != nil && conf.Type == guiutil.LibTypeURI {
		var uriConf guiutil.LibConfURI
		if confBytes, err := json.Marshal(conf.Conf); err == nil {
			if err := json.Unmarshal(confBytes, &uriConf); err == nil {
				folder, parseErr := guiutil.ParseListableURI(uriConf.URI)
				if parseErr == nil {
					state.uri = folder
					state.localRoot = guiutil.LocalPathFromURI(folder)
				}
			}
		}
	}

	a.libState = state

	confBytes, err := json.Marshal(conf)
	if err != nil {
		log.Printf("Failed to save library config: %v\n", err)
		return
	}
	a.fApp.Preferences().SetString(preferenceKeyLastLibraryConf, string(confBytes))
}

func (a *TxtlibApp) requireLocalLibraryRoot(action string) (string, bool) {
	if a.libState.localRoot != "" {
		return a.libState.localRoot, true
	}

	dialog.ShowInformation(
		action+" Unavailable",
		"This library was opened through a non-local URI, so its folder cannot be revealed in the system file browser.",
		a.window,
	)
	return "", false
}
