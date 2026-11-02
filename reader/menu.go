package main

import "log"

type menuRole int

const (
	submenuRole menuRole = iota
	appMenuRole
	editMenuRole
)

type menuItem struct {
	label  string
	accel  string
	action func()
}

type menuSpec struct {
	role  menuRole
	label string
	items []menuItem
}

func applicationMenu(app *ReaderApp) []menuSpec {
	return []menuSpec{
		{role: appMenuRole},
		{role: submenuRole, label: "File", items: []menuItem{
			{label: "Open Book…", accel: "CmdOrCtrl+O", action: func() {
				if _, err := app.OpenBookPackage(); err != nil {
					log.Println("failed to open a book package:", err)
				}
			}},
		}},
		{role: editMenuRole},
	}
}
