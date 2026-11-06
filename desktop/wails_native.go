package main

import (
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

var errNativeNotReady = util.NewError("desktop window not ready")

type wailsNative struct {
	app      *application.App
	window   *application.WebviewWindow
	notifier *lazyNotifier
}

func (w *wailsNative) attach(app *application.App, window *application.WebviewWindow) {
	w.app = app
	w.window = window
	service := notifications.New()
	service.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error == nil {
			w.bringToFront()
		}
	})
	w.notifier = &lazyNotifier{backend: wailsNotifications{service: service}}
}

func (w *wailsNative) windowFocused() bool {
	return w.window != nil && w.window.IsFocused()
}

func (w *wailsNative) notify(title, body string) error {
	if w.notifier == nil {
		return errNativeNotReady
	}
	return w.notifier.send(title, body)
}

func (w *wailsNative) bringToFront() {
	if w.window == nil {
		return
	}
	w.window.UnMinimise()
	w.window.Show()
	w.window.Focus()
}

func (w *wailsNative) openFiles(title string, filters []fileFilter) ([]string, error) {
	if w.window == nil {
		return nil, nil
	}
	dialog := w.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AttachToWindow(w.window)
	for _, f := range filters {
		dialog.AddFilter(f.label, f.pattern)
	}
	return dialog.PromptForMultipleSelection()
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

func (w *wailsNative) saveFile(title, defaultName string, filters []fileFilter) (string, error) {
	if w.window == nil {
		return "", errNativeNotReady
	}
	dialog := w.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:                title,
		Filename:             defaultName,
		CanCreateDirectories: true,
		Window:               w.window,
	})
	for _, f := range filters {
		dialog.AddFilter(f.label, f.pattern)
	}
	return dialog.PromptForSingleSelection()
}

func (w *wailsNative) showMessage(kind dialogKind, title, message string) error {
	if w.window == nil {
		return errNativeNotReady
	}
	dialog := w.app.Dialog.Error()
	if kind == warningDialog {
		dialog = w.app.Dialog.Warning()
	}
	dismissed := make(chan struct{})
	dialog.SetTitle(title).SetMessage(message).AttachToWindow(w.window)
	dialog.AddButton("OK").SetAsDefault().OnClick(func() { close(dismissed) })
	dialog.Show()
	<-dismissed
	return nil
}

func (w *wailsNative) openURL(rawURL string) {
	if w.app != nil {
		if err := w.app.Browser.OpenURL(rawURL); err != nil {
			w.app.Logger.Error("failed to open external URL", "url", rawURL, "error", err)
		}
	}
}

func (w *wailsNative) execJS(script string) {
	if w.window != nil {
		w.window.ExecJS(script)
	}
}

func (w *wailsNative) quit() {
	if w.app != nil {
		w.app.Quit()
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
