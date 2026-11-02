package main

import (
	"context"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var errNativeNotReady = util.NewError("desktop context not ready")

type wailsNative struct {
	ctx context.Context
}

func (w *wailsNative) attach(ctx context.Context) {
	w.ctx = ctx
}

func (w *wailsNative) openFiles(title string, filters []fileFilter) ([]string, error) {
	if w.ctx == nil {
		return nil, nil
	}
	return wailsruntime.OpenMultipleFilesDialog(w.ctx, wailsruntime.OpenDialogOptions{
		Title:   title,
		Filters: toWailsFilters(filters),
	})
}

func (w *wailsNative) openDirectory(title string) (string, error) {
	if w.ctx == nil {
		return "", nil
	}
	return wailsruntime.OpenDirectoryDialog(w.ctx, wailsruntime.OpenDialogOptions{Title: title})
}

func (w *wailsNative) saveFile(title, defaultName string, filters []fileFilter) (string, error) {
	if w.ctx == nil {
		return "", errNativeNotReady
	}
	return wailsruntime.SaveFileDialog(w.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         toWailsFilters(filters),
	})
}

func (w *wailsNative) showMessage(kind dialogKind, title, message string) error {
	if w.ctx == nil {
		return errNativeNotReady
	}
	dialogType := wailsruntime.ErrorDialog
	if kind == warningDialog {
		dialogType = wailsruntime.WarningDialog
	}
	_, err := wailsruntime.MessageDialog(w.ctx, wailsruntime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
	return err
}

func (w *wailsNative) openURL(rawURL string) {
	if w.ctx != nil {
		wailsruntime.BrowserOpenURL(w.ctx, rawURL)
	}
}

func (w *wailsNative) execJS(script string) {
	if w.ctx != nil {
		wailsruntime.WindowExecJS(w.ctx, script)
	}
}

func (w *wailsNative) quit() {
	if w.ctx != nil {
		wailsruntime.Quit(w.ctx)
	}
}

func toWailsFilters(filters []fileFilter) []wailsruntime.FileFilter {
	out := make([]wailsruntime.FileFilter, 0, len(filters))
	for _, f := range filters {
		out = append(out, wailsruntime.FileFilter{DisplayName: f.label, Pattern: f.pattern})
	}
	return out
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
