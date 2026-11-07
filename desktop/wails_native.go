package main

import (
	_ "embed"
	"log"
	"runtime"
	"sync"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed tray_template.png
var trayTemplateIcon []byte

var errNativeNotReady = util.NewError("desktop window not ready")

type wailsNative struct {
	app      *application.App
	window   *application.WebviewWindow
	notifier *lazyNotifier

	trayMu sync.Mutex
	tray   *application.SystemTray
}

func (w *wailsNative) attach(app *application.App, window *application.WebviewWindow) {
	w.app = app
	w.window = window
	service := notifications.New()
	service.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error == nil {
			w.showWindow()
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

func (w *wailsNative) copyText(text string) {
	if w.app != nil && !w.app.Clipboard.SetText(text) {
		log.Println("Failed to copy text to the clipboard")
	}
}

func (w *wailsNative) showWindow() {
	if w.window == nil {
		return
	}
	application.InvokeSync(w.app.Show)
	w.window.UnMinimise()
	w.window.Show().Focus()
}

func (w *wailsNative) emit(name string, data any) {
	if w.app != nil {
		w.app.Event.Emit(name, data)
	}
}

func (w *wailsNative) setTrayMenu(items []menuItem) {
	if w.app == nil || runtime.GOOS != "darwin" {
		return
	}
	w.trayMu.Lock()
	defer w.trayMu.Unlock()
	if len(items) == 0 {
		if w.tray != nil {
			w.tray.Hide()
		}
		return
	}
	menu := application.NewMenu()
	if err := addWailsMenuItems(menu, items); err != nil {
		log.Println("Failed to build LAN share menu:", err)
		return
	}
	if w.tray == nil {
		w.tray = w.app.SystemTray.New().SetTemplateIcon(trayTemplateIcon)
		w.tray.SetTooltip("PlainShelf is sharing on LAN")
	}
	w.tray.SetMenu(menu)
	w.tray.Show()
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
			if err := addWailsMenuItems(root.AddSubmenu(spec.label), spec.items); err != nil {
				return nil, err
			}
		}
	}
	return root, nil
}

func addWailsMenuItems(menu *application.Menu, items []menuItem) error {
	for _, item := range items {
		if item.separator {
			menu.AddSeparator()
			continue
		}
		entry := menu.Add(item.label).SetEnabled(!item.disabled)
		if item.accel != "" {
			if entry.SetAccelerator(item.accel).GetAccelerator() == "" {
				return util.Errorf("menu item %q: invalid accelerator %q", item.label, item.accel)
			}
		}
		if action := item.action; action != nil {
			entry.OnClick(func(*application.Context) { action() })
		}
	}
	return nil
}
