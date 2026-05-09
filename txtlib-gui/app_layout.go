package txtlibgui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

const narrowLayoutThresholdWidth float32 = 600

func (a *TxtlibApp) buildContent() fyne.CanvasObject {
	body := a.buildBody()
	return container.NewBorder(a.toolbar, a.stateBar, nil, nil, body)
}

func (a *TxtlibApp) buildBody() fyne.CanvasObject {
	if a.isNarrowLayout {
		return a.buildNarrowBody()
	}
	return a.buildWideBody()
}

func (a *TxtlibApp) buildWideBody() fyne.CanvasObject {
	leftPanel := container.NewHSplit(a.layerTreeWidget.Tree(), a.bookListComponent.List())
	leftPanel.Offset = 0.25

	body := container.NewHSplit(leftPanel, a.bookInfoWidget.Container())
	body.Offset = 0.90
	return body
}

func (a *TxtlibApp) buildNarrowBody() fyne.CanvasObject {
	infoScroll := container.NewVScroll(a.bookInfoWidget.Container())
	return container.NewAppTabs(
		container.NewTabItem("Layers", a.layerTreeWidget.Tree()),
		container.NewTabItem("Books", a.bookListComponent.List()),
		container.NewTabItem("Info", infoScroll),
	)
}

func (a *TxtlibApp) shouldUseNarrowLayout(width float32) bool {
	if width <= 0 {
		return false
	}
	return width < narrowLayoutThresholdWidth
}

func (a *TxtlibApp) startResizeWatcher() {
	a.stopResizeWatch = make(chan struct{})

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-a.stopResizeWatch:
				return
			case <-ticker.C:
				fyne.Do(func() {
					a.syncLayoutModeByWindowSize()
				})
			}
		}
	}()
}

func (a *TxtlibApp) stopResizeWatcher() {
	if a.stopResizeWatch == nil {
		return
	}

	close(a.stopResizeWatch)
	a.stopResizeWatch = nil
}

func (a *TxtlibApp) syncLayoutModeByWindowSize() {
	width := a.window.Canvas().Size().Width
	nextMode := a.shouldUseNarrowLayout(width)
	if nextMode == a.isNarrowLayout {
		return
	}

	a.isNarrowLayout = nextMode
	a.rebuildLayout()
}

func (a *TxtlibApp) rebuildLayout() {
	a.draggingBookID = ""
	a.layerTreeWidget.SetDragActive(false)
	a.window.SetContent(a.buildContent())
}

func (a *TxtlibApp) scaleDialogSize(base fyne.Size) fyne.Size {
	parent := a.window.Canvas().Size()
	if parent.Width <= 0 || parent.Height <= 0 {
		return base
	}

	width := base.Width
	height := base.Height

	maxWidth := parent.Width * 0.92
	maxHeight := parent.Height * 0.92

	if width > maxWidth {
		width = maxWidth
	}
	if height > maxHeight {
		height = maxHeight
	}

	if width < 320 {
		width = 320
	}
	if height < 200 {
		height = 200
	}

	if width > parent.Width {
		width = parent.Width
	}
	if height > parent.Height {
		height = parent.Height
	}

	return fyne.NewSize(width, height)
}
