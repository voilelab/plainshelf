package main

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

func applicationMenu(app *DesktopApp, goos string) []menuSpec {
	var specs []menuSpec
	if goos == "darwin" {
		specs = append(specs, menuSpec{role: appMenuRole})
	}
	return append(specs,
		menuSpec{role: editMenuRole},
		menuSpec{role: submenuRole, label: "View", items: []menuItem{
			{label: "Previous Page", accel: historyAccelerator(goos, "Left"), action: app.PreviousPage},
			{label: "Next Page", accel: historyAccelerator(goos, "Right"), action: app.NextPage},
			{label: "Zoom In", accel: "CmdOrCtrl+=", action: app.ZoomIn},
			{label: "Zoom Out", accel: "CmdOrCtrl+-", action: app.ZoomOut},
			{label: "Reset Zoom", accel: "CmdOrCtrl+0", action: app.ResetZoom},
		}},
	)
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

func zoomScript(action string) string {
	switch action {
	case "in":
		return "window.__plainshelfZoom && window.__plainshelfZoom('in');"
	case "out":
		return "window.__plainshelfZoom && window.__plainshelfZoom('out');"
	case "reset":
		return "window.__plainshelfZoom && window.__plainshelfZoom('reset');"
	default:
		return ""
	}
}

func historyAccelerator(goos string, key string) string {
	if goos == "darwin" {
		return "CmdOrCtrl+" + key
	}
	return "OptionOrAlt+" + key
}
