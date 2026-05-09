package component

import (
	"github.com/voilelab/plainshelf/txtlib"

	"fyne.io/fyne/v2/widget"
)

// SelectLayerWidget provides a form for selecting a target layer
type SelectLayerWidget struct {
	entry *widget.SelectEntry
	form  *widget.Form
}

// NewSelectLayerWidget creates a new layer selection widget
func NewSelectLayerWidget(allLayers []string, currentLayer string) *SelectLayerWidget {
	entry := widget.NewSelectEntry(allLayers)
	entry.SetPlaceHolder("Examples: Fiction or Fiction/Classic")
	entry.SetText(currentLayer)

	form := widget.NewForm(
		widget.NewFormItem("Target Layer", entry),
	)

	return &SelectLayerWidget{
		entry: entry,
		form:  form,
	}
}

// GetSelectedLayers returns the selected layers as a parsed Layers object
func (w *SelectLayerWidget) GetSelectedLayers() []string {
	text := w.entry.Text
	return txtlib.NewLayersFromString(text)
}

// GetForm returns the underlying form widget
func (w *SelectLayerWidget) GetForm() *widget.Form {
	return w.form
}
