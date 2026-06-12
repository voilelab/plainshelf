package main

import (
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

func newApplicationMenu(app *DesktopApp) *menu.Menu {
	root := menu.NewMenu()

	if runtime.GOOS == "darwin" {
		root.Append(menu.AppMenu())
	}

	root.Append(menu.EditMenu())

	viewMenu := root.AddSubmenu("View")

	viewMenu.AddText("Zoom In", zoomMenuAcceleratorFor(runtime.GOOS, zoomInAction), func(*menu.CallbackData) {
		app.ZoomIn()
	})
	viewMenu.AddText("Zoom Out", zoomMenuAcceleratorFor(runtime.GOOS, zoomOutAction), func(*menu.CallbackData) {
		app.ZoomOut()
	})
	viewMenu.AddText("Reset Zoom", zoomMenuAcceleratorFor(runtime.GOOS, zoomResetAction), func(*menu.CallbackData) {
		app.ResetZoom()
	})

	viewMenu.AddSeparator()

	viewMenu.AddText("Previous Page", historyMenuAcceleratorFor(runtime.GOOS, "left"), func(*menu.CallbackData) {
		app.PreviousPage()
	})
	viewMenu.AddText("Next Page", historyMenuAcceleratorFor(runtime.GOOS, "right"), func(*menu.CallbackData) {
		app.NextPage()
	})

	return root
}

func historyNavigationScript(step int) string {
	switch step {
	case -1:
		return "window.history.back();"
	case 1:
		return "window.history.forward();"
	default:
		return ""
	}
}

func historyMenuAcceleratorFor(goos string, key string) *keys.Accelerator {
	if goos == "darwin" {
		return keys.CmdOrCtrl(key)
	}

	return keys.OptionOrAlt(key)
}

func zoomMenuAcceleratorFor(goos string, action string) *keys.Accelerator {
	switch action {
	case zoomInAction:
		if goos == "windows" {
			return keys.CmdOrCtrl("plus")
		}
		return keys.Combo("=", keys.CmdOrCtrlKey, keys.ShiftKey)
	case zoomOutAction:
		return keys.CmdOrCtrl("-")
	case zoomResetAction:
		return keys.CmdOrCtrl("0")
	default:
		return nil
	}
}
