package main

import (
	"context"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var errNativeNotReady = util.NewError("reader context not ready")

type wailsNative struct {
	ctx context.Context
}

func (w *wailsNative) attach(ctx context.Context) {
	w.ctx = ctx
}

func (w *wailsNative) openDirectory(title string) (string, error) {
	if w.ctx == nil {
		return "", nil
	}
	return wailsruntime.OpenDirectoryDialog(w.ctx, wailsruntime.OpenDialogOptions{Title: title})
}

func (w *wailsNative) showError(title, message string) error {
	if w.ctx == nil {
		return errNativeNotReady
	}
	_, err := wailsruntime.MessageDialog(w.ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   title,
		Message: message,
	})
	return err
}

func (w *wailsNative) reloadApp() {
	if w.ctx != nil {
		wailsruntime.WindowReloadApp(w.ctx)
	}
}

func toWailsMenu(specs []menuSpec) (*menu.Menu, error) {
	root := menu.NewMenu()
	for _, spec := range specs {
		switch spec.role {
		case appMenuRole:
			root.Append(menu.AppMenu())
		case editMenuRole:
			root.Append(menu.EditMenu())
		default:
			sub := root.AddSubmenu(spec.label)
			for _, item := range spec.items {
				accel, err := keys.Parse(item.accel)
				if err != nil {
					return nil, util.Errorf("menu item %q: %w", item.label, err)
				}
				action := item.action
				sub.AddText(item.label, accel, func(*menu.CallbackData) { action() })
			}
		}
	}
	return root, nil
}
