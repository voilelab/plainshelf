package main

import (
	"sync"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var errNativeNotReady = util.NewError("reader window not ready")

type wailsNative struct {
	app     *application.App
	onReady func(win uint)
	onClose func(win uint)

	createMu sync.Mutex
	prepare  func(win application.Window)
}

func (w *wailsNative) attach(app *application.App, onReady, onClose func(win uint)) {
	w.app = app
	w.onReady = onReady
	w.onClose = onClose
	app.Window.OnCreate(func(win application.Window) {
		if w.prepare != nil {
			w.prepare(win)
		}
	})
}

// Wails v3.1.0 starts the webview inside NewWithOptions, after OnCreate
// callbacks, so prepare runs before the window's first asset request.
func (w *wailsNative) newWindow(prepare func(win uint)) uint {
	w.createMu.Lock()
	defer w.createMu.Unlock()

	w.prepare = func(win application.Window) {
		id := win.ID()
		prepare(id)
		win.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) { w.onClose(id) })
		win.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { w.onReady(id) })
	}
	defer func() { w.prepare = nil }()

	return w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "PlainShelf Reader",
		Width:  1100,
		Height: 900,
		URL:    "/",
	}).ID()
}

func (w *wailsNative) window(win uint) application.Window {
	if w.app == nil {
		return nil
	}
	if found, ok := w.app.Window.GetByID(win); ok {
		return found
	}
	return w.app.Window.Current()
}

func (w *wailsNative) currentWindow() uint {
	if w.app == nil {
		return 0
	}
	if current := w.app.Window.Current(); current != nil {
		return current.ID()
	}
	return 0
}

func (w *wailsNative) openDirectory(win uint, title string) (string, error) {
	window := w.window(win)
	if window == nil {
		return "", nil
	}
	return w.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		AttachToWindow(window).
		PromptForSingleSelection()
}

func (w *wailsNative) showError(win uint, title, message string) error {
	window := w.window(win)
	if window == nil {
		return errNativeNotReady
	}
	w.app.Dialog.Error().
		SetTitle(title).
		SetMessage(message).
		AttachToWindow(window).
		Show()
	return nil
}

func (w *wailsNative) reload(win uint) {
	if window := w.window(win); window != nil {
		window.SetURL("/")
	}
}

func (w *wailsNative) focus(win uint) {
	if window := w.window(win); window != nil {
		window.Show()
		window.Focus()
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
