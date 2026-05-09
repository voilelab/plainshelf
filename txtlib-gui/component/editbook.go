package component

import (
	"io"
	"path"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
	"github.com/voilelab/plainshelf/txtlib-gui/filedialog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type EditBookWindow struct {
	dlg           dialog.Dialog
	parent        fyne.Window
	book          *txtlib.Book
	sizeDialog    func(base fyne.Size) fyne.Size
	onSave        func(*txtlib.Book)
	titleEntry    *widget.Entry
	authorsEntry  *widget.Entry
	languageEntry *widget.Entry
	publishedAt   *widget.Entry
	commentsEntry *widget.Entry
	coverImage    *canvas.Image
	pendingCover  []byte
	pendingExt    string
	removeCover   bool
}

func NewEditBookWindow(parent fyne.Window, book *txtlib.Book, sizeDialog func(base fyne.Size) fyne.Size, onSave func(*txtlib.Book)) *EditBookWindow {
	meta := book.GetMeta()

	titleEntry := widget.NewEntry()
	titleEntry.SetText(meta.Title)

	authorsEntry := widget.NewEntry()
	authorsEntry.SetText(strings.Join(meta.Authors, ", "))

	languageEntry := widget.NewEntry()
	languageEntry.SetText(meta.Language)

	publishedAtEntry := widget.NewEntry()
	publishedAtEntry.SetPlaceHolder("YYYY-MM-DD or RFC3339 (leave empty to clear)")
	publishedAtEntry.SetText(formatPublishedAt(meta.PublishedAt))
	publishedAtPicker := widget.NewButton("Pick Date", nil)

	commentsEntry := widget.NewMultiLineEntry()
	commentsEntry.SetText(meta.Comments)
	commentsEntry.SetMinRowsVisible(5)

	ew := &EditBookWindow{
		parent:        parent,
		book:          book,
		sizeDialog:    sizeDialog,
		onSave:        onSave,
		titleEntry:    titleEntry,
		authorsEntry:  authorsEntry,
		languageEntry: languageEntry,
		publishedAt:   publishedAtEntry,
		commentsEntry: commentsEntry,
		coverImage:    canvas.NewImageFromResource(theme.DocumentIcon()),
	}
	ew.coverImage.SetMinSize(fyne.NewSize(180, 220))
	ew.coverImage.FillMode = canvas.ImageFillContain
	ew.loadCurrentCover()
	publishedAtPicker.OnTapped = ew.showPublishedAtCalendar

	itemAuthors := widget.NewFormItem("Authors", authorsEntry)
	itemAuthors.HintText = "comma separated, e.g. Author One, Author Two"

	form := widget.NewForm(
		widget.NewFormItem("Title", titleEntry),
		itemAuthors,
		widget.NewFormItem("Language", languageEntry),
		widget.NewFormItem("Published At", container.NewBorder(nil, nil, nil, publishedAtPicker, publishedAtEntry)),
		widget.NewFormItem("Comments", commentsEntry),
	)

	coverButtons := container.NewGridWithColumns(2,
		widget.NewButton("Change", ew.changeCover),
		widget.NewButton("Remove", ew.removeCoverAction),
	)

	leftPanel := container.NewVBox(
		ew.coverImage,
		coverButtons,
	)

	content := container.NewGridWithColumns(2, leftPanel, form)

	ew.dlg = dialog.NewCustomConfirm("Edit Book Metadata", "Save", "Cancel", content, func(confirmed bool) {
		if !confirmed {
			return
		}
		ew.save()
	}, parent)
	ew.dlg.Resize(ew.scaleDialogSize(fyne.NewSize(700, 440)))

	return ew
}

func (ew *EditBookWindow) scaleDialogSize(base fyne.Size) fyne.Size {
	if ew.sizeDialog == nil {
		return base
	}
	return ew.sizeDialog(base)
}

func (ew *EditBookWindow) Show() {
	ew.dlg.Show()
}

func (ew *EditBookWindow) save() {
	meta := ew.book.GetMeta()
	meta.Title = strings.TrimSpace(ew.titleEntry.Text)
	meta.Authors = parseAuthors(ew.authorsEntry.Text)
	meta.Language = strings.TrimSpace(ew.languageEntry.Text)
	publishedAt, err := parsePublishedAt(ew.publishedAt.Text)
	if err != nil {
		dialog.ShowError(err, ew.parent)
		return
	}
	meta.PublishedAt = publishedAt
	meta.Comments = strings.TrimSpace(ew.commentsEntry.Text)
	if ew.removeCover {
		meta.Cover = ""
	}

	err = ew.book.SetMeta(meta)
	if err != nil {
		dialog.ShowError(err, ew.parent)
		return
	}

	if len(ew.pendingCover) > 0 {
		err = ew.book.SetCover(ew.pendingCover, ew.pendingExt)
		if err != nil {
			dialog.ShowError(err, ew.parent)
			return
		}
	}

	if ew.onSave != nil {
		ew.onSave(ew.book)
	}
}

func (ew *EditBookWindow) loadCurrentCover() {
	coverData, _, err := ew.book.OpenCover()
	if err != nil || len(coverData) == 0 {
		ew.coverImage.Resource = theme.DocumentIcon()
		ew.coverImage.Refresh()
		return
	}

	ew.coverImage.Resource = fyne.NewStaticResource(ew.book.Title()+"_cover_preview", coverData)
	ew.coverImage.Refresh()
}

func (ew *EditBookWindow) changeCover() {
	filedialog.OpenFileDialog("Select a cover image", []string{"*.jpg", "*.jpeg", "*.png", "*.gif"}, func(img fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ew.parent)
			return
		}

		if img == nil {
			return
		}

		defer img.Close()

		bs, err := io.ReadAll(img)
		if err != nil {
			dialog.ShowError(err, ew.parent)
			return
		}

		ew.pendingCover = bs
		ew.pendingExt = path.Ext(img.URI().Path())
		ew.removeCover = false
		ew.coverImage.Resource = fyne.NewStaticResource(ew.book.Title()+"_cover_pending", bs)
		ew.coverImage.Refresh()
	})
}

func (ew *EditBookWindow) removeCoverAction() {
	ew.pendingCover = nil
	ew.pendingExt = ""
	ew.removeCover = true
	ew.coverImage.Resource = theme.DocumentIcon()
	ew.coverImage.Refresh()
}

func parseAuthors(raw string) []string {
	parts := strings.Split(raw, ",")
	authors := make([]string, 0, len(parts))
	for _, part := range parts {
		author := strings.TrimSpace(part)
		if author == "" {
			continue
		}
		authors = append(authors, author)
	}

	return authors
}

func formatPublishedAt(t util.JSONTime) string {
	if t.IsZero() {
		return ""
	}

	return time.Time(t).Format(time.RFC3339)
}

func parsePublishedAt(raw string) (util.JSONTime, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return util.JSONTime(time.Time{}), nil
	}

	if parsed, err := time.Parse(time.RFC3339, input); err == nil {
		return util.JSONTime(parsed), nil
	}

	if parsed, err := time.Parse("2006-01-02", input); err == nil {
		return util.JSONTime(parsed), nil
	}

	return util.JSONTime(time.Time{}), util.Errorf("invalid Published At: %q (use YYYY-MM-DD or RFC3339)", input)
}

func (ew *EditBookWindow) showPublishedAtCalendar() {
	current := time.Now()
	if parsed, err := parsePublishedAt(ew.publishedAt.Text); err == nil && !parsed.IsZero() {
		current = time.Time(parsed)
	}

	selected := current
	calendar := widget.NewCalendar(current, func(t time.Time) {
		selected = t
	})

	dlg := dialog.NewCustomConfirm("Select Published Date", "Use Date", "Cancel", calendar, func(confirmed bool) {
		if !confirmed {
			return
		}
		ew.publishedAt.SetText(selected.Format("2006-01-02"))
	}, ew.parent)
	dlg.Resize(ew.scaleDialogSize(fyne.NewSize(360, 320)))
	dlg.Show()
}
