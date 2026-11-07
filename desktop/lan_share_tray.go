package main

import "log"

const lanShareChangedEvent = "lan-share-changed"

func lanShareTrayItems(st LANShareStatus, app *DesktopApp) []menuItem {
	if !st.Running {
		return nil
	}
	items := []menuItem{{label: "Sharing on LAN", disabled: true}}
	shared := 0
	for _, sh := range st.Shelves {
		if sh.Shared {
			items = append(items, menuItem{label: sh.Name, disabled: true})
			shared++
		}
	}
	if shared == 0 {
		items = append(items, menuItem{label: "No shelves shared", disabled: true})
	}
	url := st.URL
	return append(items,
		menuItem{label: url, disabled: true},
		menuItem{separator: true},
		menuItem{label: "Copy URL", action: func() { app.native.copyText(url) }},
		menuItem{label: "Stop Sharing", action: app.stopLANShare},
		menuItem{separator: true},
		menuItem{label: "Open PlainShelf", action: app.native.showWindow},
	)
}

func (a *DesktopApp) lanShareChanged(st LANShareStatus) {
	a.native.emit(lanShareChangedEvent, st)
	a.native.setTrayMenu(lanShareTrayItems(st, a))
}

func (a *DesktopApp) watchLANShare() {
	if a.lanShare == nil {
		return
	}
	a.lanShare.watch(a.lanShareChanged)
	st, err := a.lanShare.status()
	if err != nil {
		log.Println("Failed to read LAN share status:", err)
		return
	}
	a.native.setTrayMenu(lanShareTrayItems(st, a))
}

func (a *DesktopApp) stopLANShare() {
	if a.lanShare == nil {
		return
	}
	if err := a.lanShare.disable(); err != nil {
		log.Println("Failed to stop LAN share:", err)
	}
}
