package main

import (
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var errNativeNotReady = util.NewError("reader window not ready")

type wailsNative struct {
	app    *application.App
	window *application.WebviewWindow
}

func (w *wailsNative) attach(app *application.App, window *application.WebviewWindow) {
	w.app = app
	w.window = window
}

func (w *wailsNative) openDirectory(title string) (string, error) {
	if w.window == nil {
		return "", nil
	}
	return w.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		AttachToWindow(w.window).
		PromptForSingleSelection()
}

func (w *wailsNative) showError(title, message string) error {
	if w.window == nil {
		return errNativeNotReady
	}
	w.app.Dialog.Error().
		SetTitle(title).
		SetMessage(message).
		AttachToWindow(w.window).
		Show()
	return nil
}

func (w *wailsNative) reloadApp() {
	if w.window != nil {
		w.window.SetURL("/")
	}
}

func (w *wailsNative) focus() {
	if w.window != nil {
		w.window.Show()
		w.window.Focus()
	}
}

func toWailsMenu(specs []menuSpec) (*application.Menu, error) {
	root := application.NewMenu()
	for _, spec := range specs {
		switch spec.role {
		case appMenuRole:
			root.AddRole(application.AppMenu)
		case editMenuRole:
			root.AddRole(application.EditMenu)
		default:
			sub := root.AddSubmenu(spec.label)
			for _, item := range spec.items {
				entry := sub.Add(item.label).SetAccelerator(item.accel)
				if entry.GetAccelerator() == "" {
					return nil, util.Errorf("menu item %q: invalid accelerator %q", item.label, item.accel)
				}
				action := item.action
				entry.OnClick(func(*application.Context) { action() })
			}
		}
	}
	return root, nil
}
