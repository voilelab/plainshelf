package component

import (
	"fmt"
	"io/fs"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type TxtViewWindow struct {
	window        fyne.Window
	fp            fs.File
	content       *fyne.Container
	contentText   *widget.Entry
	indexWidget   *widget.Label
	chapterReader *util.ChapterReader
}

func NewTxtViewWindow(app fyne.App, snapshot *txtlib.Snapshot, initialSize fyne.Size) *TxtViewWindow {
	contentText := widget.NewMultiLineEntry()
	contentText.Wrapping = fyne.TextWrapWord
	contentText.TextStyle = fyne.TextStyle{Monospace: true}

	fp, err := snapshot.OpenSource()
	if err != nil {
		contentText.SetText("Failed to open book content: " + err.Error())
	}

	tv := &TxtViewWindow{
		fp:            fp,
		content:       nil, // Will be set later
		contentText:   contentText,
		indexWidget:   widget.NewLabel("Chapter: 1"), // Example initial chapter index
		chapterReader: util.NewChapterReader(fp, 20), // Example line count
	}

	contentText.SetText(tv.chapterReader.Current())

	operatorBar := container.NewHBox(
		widget.NewButton("Previous Chapter", tv.prev),
		tv.indexWidget,
		widget.NewButton("Next Chapter", tv.next),
	)

	contentScroll := container.NewScroll(contentText)
	tv.content = container.NewBorder(nil, operatorBar, nil, nil, contentScroll)

	window := app.NewWindow("TxtView")
	window.SetContent(tv.content)
	if initialSize.Width > 0 && initialSize.Height > 0 {
		window.Resize(initialSize)
	} else {
		window.Resize(fyne.NewSize(600, 400))
	}
	window.SetOnClosed(tv.Close)

	window.Canvas().SetOnTypedKey(func(keyEvent *fyne.KeyEvent) {
		switch keyEvent.Name {
		case fyne.KeyLeft:
			tv.prev()
		case fyne.KeyRight:
			tv.next()
		}
	})

	tv.window = window

	return tv
}

func (tv *TxtViewWindow) Show() {
	tv.window.Show()
}

func (tv *TxtViewWindow) Close() {
	if tv.fp != nil {
		tv.fp.Close()
	}
}

func (tv *TxtViewWindow) prev() {
	tv.chapterReader.Prev()
	tv.contentText.SetText(tv.chapterReader.Current())
	tv.indexWidget.SetText(fmt.Sprintf("Chapter: %d", tv.chapterReader.Index()+1))
}

func (tv *TxtViewWindow) next() {
	tv.chapterReader.Next()
	tv.contentText.SetText(tv.chapterReader.Current())
	tv.indexWidget.SetText(fmt.Sprintf("Chapter: %d", tv.chapterReader.Index()+1))
}
