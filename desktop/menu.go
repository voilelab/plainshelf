package main

import (
	"fmt"
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

	viewMenu.AddText("Previous Page", historyMenuAcceleratorFor(runtime.GOOS, "left"), func(*menu.CallbackData) {
		app.PreviousPage()
	})
	viewMenu.AddText("Next Page", historyMenuAcceleratorFor(runtime.GOOS, "right"), func(*menu.CallbackData) {
		app.NextPage()
	})

	viewMenu.AddSeparator()

	viewMenu.AddText("Zoom In", keys.CmdOrCtrl("+"), func(*menu.CallbackData) {
		app.ZoomIn()
	})
	viewMenu.AddText("Zoom Out", keys.CmdOrCtrl("-"), func(*menu.CallbackData) {
		app.ZoomOut()
	})
	viewMenu.AddText("Reset Zoom", keys.CmdOrCtrl("0"), func(*menu.CallbackData) {
		app.ResetZoom()
	})

	return root
}

func zoomScript(factor float64) string {
	return fmt.Sprintf("document.documentElement.style.zoom = '%.2f';", factor)
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
