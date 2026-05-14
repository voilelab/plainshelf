package component

import (
	"path"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/voilelab/plainshelf/shelf"
)

const layerTreeRootID = "__all_layers__"

type LayerTreeWidget struct {
	tree       *widget.Tree
	children   map[string][]string
	titles     map[string]string
	selectedID string
	onSelect   func(string)
	onHover    func(string)
	dragActive bool
	hoveredID  string
	suppress   bool
}

type layerTreeNodeLabel struct {
	widget.Label
	uid      string
	title    string
	hover    func(string)
	hoverOut func(string)
}

func newLayerTreeNodeLabel(onHover func(string), onHoverOut func(string)) *layerTreeNodeLabel {
	n := &layerTreeNodeLabel{hover: onHover, hoverOut: onHoverOut}
	n.ExtendBaseWidget(n)
	return n
}

func (n *layerTreeNodeLabel) SetNode(uid string, title string, highlighted bool) {
	n.uid = uid
	n.title = title
	n.TextStyle = fyne.TextStyle{Bold: highlighted}
	if highlighted {
		n.SetText("-> " + title)
		return
	}
	n.SetText(title)
}

func (n *layerTreeNodeLabel) MouseIn(_ *desktop.MouseEvent) {
	if n.hover != nil {
		n.hover(n.uid)
	}
}

func (n *layerTreeNodeLabel) MouseMoved(_ *desktop.MouseEvent) {
	if n.hover != nil {
		n.hover(n.uid)
	}
}

func (n *layerTreeNodeLabel) MouseOut() {
	if n.hoverOut != nil {
		n.hoverOut(n.uid)
	}
}

func NewLayerTreeWidget(onSelect func(string), onHover func(string)) *LayerTreeWidget {
	w := &LayerTreeWidget{
		children: map[string][]string{
			layerTreeRootID: {},
		},
		titles: map[string]string{
			layerTreeRootID: "All Layers",
		},
		selectedID: layerTreeRootID,
		onSelect:   onSelect,
		onHover:    onHover,
	}

	w.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			children := w.children[string(uid)]
			ret := make([]widget.TreeNodeID, len(children))
			for i, child := range children {
				ret[i] = widget.TreeNodeID(child)
			}
			return ret
		},
		func(uid widget.TreeNodeID) bool {
			return len(w.children[string(uid)]) > 0
		},
		func(_ bool) fyne.CanvasObject {
			return newLayerTreeNodeLabel(w.setHoveredLayer, w.clearHoveredLayer)
		},
		func(uid widget.TreeNodeID, _ bool, obj fyne.CanvasObject) {
			isHovered := w.dragActive && w.hoveredID == string(uid)
			obj.(*layerTreeNodeLabel).SetNode(string(uid), w.titles[string(uid)], isHovered)
		},
	)

	w.tree.Root = widget.TreeNodeID(layerTreeRootID)
	w.tree.OnSelected = func(uid widget.TreeNodeID) {
		w.selectedID = string(uid)
		if w.suppress || w.onSelect == nil {
			return
		}
		w.onSelect(w.selectedID)
	}
	w.tree.OpenBranch(widget.TreeNodeID(layerTreeRootID))

	return w
}

func (w *LayerTreeWidget) Tree() *widget.Tree {
	return w.tree
}

func (w *LayerTreeWidget) SelectedLayer() shelf.Layers {
	if w.selectedID == layerTreeRootID {
		return nil
	}
	return shelf.NewLayersFromString(w.selectedID)
}

func (w *LayerTreeWidget) SetDragActive(active bool) {
	if w.dragActive == active {
		return
	}

	prev := w.hoveredID
	w.dragActive = active
	if !active {
		w.hoveredID = ""
		if w.onHover != nil {
			w.onHover("")
		}
	}

	if prev != "" {
		w.tree.RefreshItem(widget.TreeNodeID(prev))
	}
	if w.hoveredID != "" {
		w.tree.RefreshItem(widget.TreeNodeID(w.hoveredID))
	}
}

func (w *LayerTreeWidget) HoveredLayer() shelf.Layers {
	if w.hoveredID == layerTreeRootID {
		return nil
	}
	return shelf.NewLayersFromString(w.hoveredID)
}

func (w *LayerTreeWidget) setHoveredLayer(layerID string) {
	if !w.dragActive || layerID == w.hoveredID {
		return
	}

	prev := w.hoveredID
	w.hoveredID = layerID
	if w.onHover != nil {
		w.onHover(layerID)
	}

	if prev != "" {
		w.tree.RefreshItem(widget.TreeNodeID(prev))
	}
	w.tree.RefreshItem(widget.TreeNodeID(layerID))
}

func (w *LayerTreeWidget) clearHoveredLayer(layerID string) {
	if !w.dragActive || w.hoveredID != layerID {
		return
	}

	w.hoveredID = ""
	if w.onHover != nil {
		w.onHover("")
	}
	w.tree.RefreshItem(widget.TreeNodeID(layerID))
}

func (w *LayerTreeWidget) SelectLayer(layerID string) {
	if layerID == "" {
		layerID = layerTreeRootID
	}

	if _, ok := w.titles[layerID]; !ok {
		layerID = layerTreeRootID
	}

	w.selectedID = layerID
	w.tree.Select(widget.TreeNodeID(layerID))
}

func (w *LayerTreeWidget) SetLayers(allLayers []shelf.Layers) {
	children := map[string][]string{
		layerTreeRootID: {},
	}
	titles := map[string]string{
		layerTreeRootID: "All Layers",
	}

	childSets := map[string]map[string]struct{}{
		layerTreeRootID: {},
	}

	for _, layer := range allLayers {
		parts := layer
		parentID := layerTreeRootID
		currentPath := ""
		for _, part := range parts {
			if part == "" {
				continue
			}

			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = path.Join(currentPath, part)
			}

			if _, ok := childSets[parentID]; !ok {
				childSets[parentID] = map[string]struct{}{}
			}
			childSets[parentID][currentPath] = struct{}{}
			if _, ok := childSets[currentPath]; !ok {
				childSets[currentPath] = map[string]struct{}{}
			}

			titles[currentPath] = part
			parentID = currentPath
		}
	}

	for parentID, entries := range childSets {
		if len(entries) == 0 {
			children[parentID] = []string{}
			continue
		}

		kids := make([]string, 0, len(entries))
		for kid := range entries {
			kids = append(kids, kid)
		}
		sort.Strings(kids)
		children[parentID] = kids
	}

	w.children = children
	w.titles = titles
	if _, ok := w.titles[w.selectedID]; !ok {
		w.selectedID = layerTreeRootID
	}

	w.suppress = true
	w.tree.Refresh()
	w.tree.Select(widget.TreeNodeID(w.selectedID))
	w.tree.OpenBranch(widget.TreeNodeID(layerTreeRootID))
	w.suppress = false
}
