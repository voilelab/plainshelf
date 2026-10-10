package main

import (
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

type DesktopShelfDetails struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	ScanInterval      string `json:"scan_interval"`
	BookCheckInterval string `json:"book_check_interval"`
	ReadOnly          bool   `json:"read_only"`
}

// AddShelfParams is a struct rather than a positional argument list because
// Wails binds it by field name: every further per-shelf setting the UI exposes
// is a field here rather than one more anonymous positional argument.
type AddShelfParams struct {
	Name              string `json:"name"`
	LibRoot           string `json:"libRoot"`
	ScanInterval      string `json:"scanInterval"`
	BookCheckInterval string `json:"bookCheckInterval"`
	ReadOnly          bool   `json:"readOnly"`
}

// ModifyShelfParams carries the fields the modify-shelf form submits; see
// AddShelfParams for why this is a struct. LibRoot is absent because a shelf's
// directory is fixed once it is registered.
type ModifyShelfParams struct {
	ShelfID           string `json:"shelfID"`
	Name              string `json:"name"`
	ScanInterval      string `json:"scanInterval"`
	BookCheckInterval string `json:"bookCheckInterval"`
	ReadOnly          bool   `json:"readOnly"`
}

// ShelfIDPreview is what the add-shelf form shows while the user types: the id
// the shelf would be created with, and the directory it would be created in if
// the user never picks one.
type ShelfIDPreview struct {
	ID string `json:"id"`
	// DefaultPath is the shelf directory the form submits as lib_root unless the
	// user typed or browsed to one. It is only ever a suggestion: AddShelf writes
	// whatever lib_root it is given, so what the form shows is what shelves.json
	// records.
	DefaultPath string `json:"defaultPath"`
}

// PreviewShelfID reports the shelf id AddShelf would assign to a shelf created
// with the given name right now, including the uniqueness suffix, along with
// the default directory such a shelf would live in. The frontend shows both
// live as the user types so a name that slugifies to nothing — a purely
// non-ASCII name such as "小說" — visibly becomes "shelf" before the shelf is
// created and its id frozen as the reading-progress key. An empty or
// whitespace-only name has no preview and returns zero values.
//
// The id and the path are derived together because the path is named after the
// id: the uniqueness suffix that keeps two shelves' ids apart keeps their
// default directories apart as well.
func (a *DesktopApp) PreviewShelfID(name string) (ShelfIDPreview, error) {
	if strings.TrimSpace(name) == "" {
		return ShelfIDPreview{}, nil
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return ShelfIDPreview{}, util.Errorf("loading shelf config: %w", err)
	}

	existingIDs := map[string]bool{}
	for _, entry := range conf.Shelves {
		existingIDs[entry.ID] = true
	}

	id := generateDesktopShelfID(name, existingIDs)
	return ShelfIDPreview{
		ID:          id,
		DefaultPath: defaultDesktopShelfDir(a.shelvesConfigPath, id),
	}, nil
}

// AddShelf registers a new shelf and persists it to shelves.json.
//
// readOnly opens the shelf without writing to it at all (shelf.ShelfConf):
// lib_root must already exist, because a read-only shelf is never created, and
// neither the lock file nor the exported book cache is written.
func (a *DesktopApp) AddShelf(params AddShelfParams) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	name := strings.TrimSpace(params.Name)
	if name == "" {
		return util.Errorf("shelf name cannot be empty")
	}

	normalizedLibRoot, err := normalizeDesktopShelfDirectory(params.LibRoot)
	if err != nil {
		return util.Errorf("%w", err)
	}

	scanInterval := strings.TrimSpace(params.ScanInterval)
	bookCheckInterval := strings.TrimSpace(params.BookCheckInterval)

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	existingIDs := map[string]bool{}
	for _, entry := range conf.Shelves {
		existingIDs[entry.ID] = true
	}

	id := generateDesktopShelfID(name, existingIDs)

	entry := desktopShelfEntry{
		ID:                id,
		Name:              name,
		LibRoot:           normalizedLibRoot,
		ScanInterval:      scanInterval,
		BookCheckInterval: bookCheckInterval,
		ReadOnly:          params.ReadOnly,
	}

	err = a.app.AddShelf(toShelfConfWithID(entry))
	if err != nil {
		return util.Errorf("registering shelf: %w", err)
	}

	conf.Shelves = append(conf.Shelves, entry)
	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		if removeErr := a.app.RemoveShelf(id); removeErr != nil {
			return util.Errorf("saving shelf config: %w; rolling back runtime shelf: %v", err, removeErr)
		}
		return util.Errorf("saving shelf config: %w", err)
	}

	return nil
}

func (a *DesktopApp) GetShelfDetails(shelfID string) (*DesktopShelfDetails, error) {
	if a.app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return nil, util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return nil, util.Errorf("loading shelf config: %w", err)
	}

	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			return &DesktopShelfDetails{
				ID:                entry.ID,
				Name:              entry.Name,
				Path:              entry.LibRoot,
				ScanInterval:      entry.ScanInterval,
				BookCheckInterval: entry.BookCheckInterval,
				ReadOnly:          entry.ReadOnly,
			}, nil
		}
	}

	return nil, util.Errorf("shelf with ID %q not found", shelfID)
}

// ModifyShelf applies edited settings to an existing shelf.
//
// Turning readOnly on stops the shelf being written to; turning it off restores
// writes. Neither direction may become one-way: what this method edits is
// shelves.json in the desktop data directory, which is outside every shelf, so
// a shelf's own read_only has no say over whether its settings can be changed.
//
// That is worth keeping deliberately, because it currently holds by accident:
// the desktop reaches this method through a Wails binding rather than the HTTP
// handler, so server.App.rejectReadOnlyWrite never sees the request. Were shelf
// management ever moved onto the HTTP API, a server started with
// app_conf.read_only would refuse the very request that turns read-only off,
// and the only way back would be to edit a config file by hand. Whatever serves
// this edit has to stay reachable while the shelf it edits is read-only.
func (a *DesktopApp) ModifyShelf(params ModifyShelfParams) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	shelfID := strings.TrimSpace(params.ShelfID)
	if shelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	name := strings.TrimSpace(params.Name)
	if name == "" {
		return util.Errorf("shelf name cannot be empty")
	}

	scanInterval := strings.TrimSpace(params.ScanInterval)
	bookCheckInterval := strings.TrimSpace(params.BookCheckInterval)

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	var found *desktopShelfEntry
	for i := range conf.Shelves {
		if conf.Shelves[i].ID == shelfID {
			found = &conf.Shelves[i]
			break
		}
	}
	if found == nil {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}

	previous := *found

	updated := previous
	updated.Name = name
	updated.ScanInterval = scanInterval
	updated.BookCheckInterval = bookCheckInterval
	updated.ReadOnly = params.ReadOnly

	if err := a.app.UpdateShelf(toShelfConfWithID(updated)); err != nil {
		return util.Errorf("updating shelf: %w", err)
	}

	*found = updated

	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		if rollbackErr := a.app.UpdateShelf(toShelfConfWithID(previous)); rollbackErr != nil {
			return util.Errorf("saving shelf config: %w; rolling back runtime shelf: %v", err, rollbackErr)
		}
		return util.Errorf("saving shelf config: %w", err)
	}

	return nil
}

func (a *DesktopApp) RemoveShelf(shelfID string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	// DeleteFunc mutates its argument in place, so clone first: conf.Shelves is
	// read again in the length check below. The check treats removing several
	// duplicate IDs the same as removing one, exactly as the previous found-bool
	// loop did.
	newShelves := slices.DeleteFunc(slices.Clone(conf.Shelves), func(entry desktopShelfEntry) bool {
		return entry.ID == shelfID
	})
	if len(newShelves) == len(conf.Shelves) {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}

	conf.Shelves = newShelves
	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		return util.Errorf("saving shelf config: %w", err)
	}

	if err := a.app.RemoveShelf(shelfID); err != nil {
		return util.Errorf("removing shelf from runtime: %w", err)
	}

	return nil
}
