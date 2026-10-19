package main

import (
	"log"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

type DesktopShelfDetails struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	ScanInterval      string `json:"scan_interval"`
	BookCheckInterval string `json:"book_check_interval"`
	ReadOnly          bool   `json:"read_only"`
	// Source is the virtual shelf's source type, empty for a local shelf; URL
	// and Username are then its address and account. The secret is never
	// returned.
	Source   string `json:"source"`
	URL      string `json:"url"`
	Username string `json:"username"`

	// OpenError is a safe summary; backend errors can contain credentials.
	OpenError string `json:"open_error"`
}

// ShelfSourceParams names a virtual shelf's source as the add-shelf form
// typed it. Secret goes to the OS keychain only.
type ShelfSourceParams struct {
	// Source is "plainshelf" or "webdav"; empty means a local shelf.
	Source   string `json:"source"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
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
	// Remote, when its Source is set, adds a virtual shelf instead of LibRoot.
	Remote ShelfSourceParams `json:"remote"`
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

	id := generateDesktopShelfID(name, conf.existingIDs())
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
	if params.Remote.Source != "" {
		return a.addVirtualShelf(name, params)
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

	id := generateDesktopShelfID(name, conf.existingIDs())

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

// addVirtualShelf adds a plainshelf or webdav shelf. The secret is stored in
// the keychain under the new shelf's ID before the shelf opens, since opening
// resolves it, and removed again if the shelf is not saved.
func (a *DesktopApp) addVirtualShelf(name string, params AddShelfParams) error {
	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}
	id := generateDesktopShelfID(name, conf.existingIDs())

	remote := params.Remote
	src := &shelf.SourceConf{
		Type:     remote.Source,
		URL:      strings.TrimSpace(remote.URL),
		Username: strings.TrimSpace(remote.Username),
	}
	secret := strings.TrimSpace(remote.Secret)
	if secret != "" {
		src.Credential = hostCredentialPrefix + id
	}
	if src.Type != shelf.SourcePlainShelf && src.Type != shelf.SourceWebDAV {
		return util.Errorf("source type %q is not available on the desktop", src.Type)
	}
	if err := src.Validate(); err != nil {
		return util.Errorf("%w", err)
	}

	entry := desktopShelfEntry{
		ID:                id,
		Name:              name,
		Source:            src,
		ScanInterval:      strings.TrimSpace(params.ScanInterval),
		BookCheckInterval: strings.TrimSpace(params.BookCheckInterval),
		// What the shelf opens as anyway; recorded so ModifyShelf matches it.
		ReadOnly: true,
	}

	secrets := a.secretStore()
	if secret != "" {
		if err := secrets.Set(id, secret); err != nil {
			return util.Errorf("storing the shelf's secret in the keychain: %w", err)
		}
	}
	forgetSecret := func() {
		if secret == "" {
			return
		}
		if err := secrets.Delete(id); err != nil {
			log.Println("Failed to remove a shelf secret from the keychain:", err)
		}
	}

	if err := a.app.AddShelf(toShelfConfWithID(entry)); err != nil {
		forgetSecret()
		return util.Errorf("registering shelf: %w", err)
	}

	conf.VirtualShelves = append(conf.VirtualShelves, entry)
	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		removeErr := a.app.RemoveShelf(id)
		forgetSecret()
		if removeErr != nil {
			return util.Errorf("saving shelf config: %w; rolling back runtime shelf: %v", err, removeErr)
		}
		return util.Errorf("saving shelf config: %w", err)
	}
	return nil
}

// CheckShelfSource probes the source the add-shelf form describes, with the
// secret it holds, before anything is stored. A Code of SOURCE_UNAUTHORIZED is
// a refused credential and SOURCE_UNAVAILABLE no connection.
func (a *DesktopApp) CheckShelfSource(params ShelfSourceParams) (server.SourceCheck, error) {
	check, err := server.CheckSource(params.Source, strings.TrimSpace(params.URL),
		strings.TrimSpace(params.Username), strings.TrimSpace(params.Secret))
	if err != nil {
		return server.SourceCheck{}, util.Errorf("%w", err)
	}
	return check, nil
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

	entry := conf.find(shelfID)
	if entry == nil {
		return nil, util.Errorf("shelf with ID %q not found", shelfID)
	}
	return a.shelfDetails(*entry), nil
}

func (a *DesktopApp) shelfDetails(entry desktopShelfEntry) *DesktopShelfDetails {
	details := &DesktopShelfDetails{
		ID:                entry.ID,
		Name:              entry.Name,
		Path:              entry.LibRoot,
		ScanInterval:      entry.ScanInterval,
		BookCheckInterval: entry.BookCheckInterval,
		ReadOnly:          entry.ReadOnly,
	}
	if entry.Source != nil {
		details.Source, details.URL, details.Username = entry.Source.Type, entry.Source.URL, entry.Source.Username
		if _, open := a.app.ShelfManager().GetShelf(entry.ID); !open {
			details.OpenError = "The virtual shelf could not be opened. Check the connection and unlock the keychain, then retry."
		}
	}
	return details
}

// ListShelfDetails includes configured virtual shelves that never opened. This
// management list is separate from the runtime list used to select a shelf.
func (a *DesktopApp) ListShelfDetails() ([]DesktopShelfDetails, error) {
	if a.app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}
	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return nil, util.Errorf("loading shelf config: %w", err)
	}
	details := make([]DesktopShelfDetails, 0, len(conf.Shelves)+len(conf.VirtualShelves))
	for _, entry := range append(slices.Clone(conf.Shelves), conf.VirtualShelves...) {
		details = append(details, *a.shelfDetails(entry))
	}
	return details, nil
}

// RetryShelf opens a configured virtual shelf without restarting the desktop.
// Never send raw source/keychain errors to the UI: they may contain secrets.
func (a *DesktopApp) RetryShelf(shelfID string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}
	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}
	entry := conf.find(strings.TrimSpace(shelfID))
	if entry == nil || entry.Source == nil {
		return util.NewError("configured virtual shelf not found")
	}
	if _, open := a.app.ShelfManager().GetShelf(entry.ID); open {
		return nil
	}
	if err := a.app.AddShelf(toShelfConfWithID(*entry)); err != nil {
		return util.NewError("The virtual shelf could not be opened. Check the connection and unlock the keychain, then retry.")
	}
	return nil
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

	found := conf.find(shelfID)
	if found == nil {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}

	previous := *found

	updated := previous
	updated.Name = name
	updated.ScanInterval = scanInterval
	updated.BookCheckInterval = bookCheckInterval
	// A virtual shelf has no write path to switch on.
	updated.ReadOnly = params.ReadOnly || updated.Source != nil

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

	removed := conf.find(shelfID)
	if removed == nil {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}
	// Read before DeleteFunc, which overwrites the slots removed points into.
	source := removed.Source

	// First, while the entry still exists: a keychain that refuses leaves the
	// shelf in place, so removing it again retries instead of orphaning the
	// secret.
	if source != nil && strings.HasPrefix(source.Credential, hostCredentialPrefix) {
		if err := a.secretStore().Delete(strings.TrimPrefix(source.Credential, hostCredentialPrefix)); err != nil {
			return util.Errorf("removing the shelf's secret from the keychain: %w", err)
		}
	}

	isShelf := func(entry desktopShelfEntry) bool { return entry.ID == shelfID }
	conf.Shelves = slices.DeleteFunc(conf.Shelves, isShelf)
	conf.VirtualShelves = slices.DeleteFunc(conf.VirtualShelves, isShelf)

	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		return util.Errorf("saving shelf config: %w", err)
	}

	// A virtual shelf that failed to open at startup is in the config only.
	if _, open := a.app.ShelfManager().GetShelf(shelfID); open || source == nil {
		if err := a.app.RemoveShelf(shelfID); err != nil {
			return util.Errorf("removing shelf from runtime: %w", err)
		}
	}

	if a.lanShare != nil {
		if err := a.lanShare.forgetShelf(shelfID); err != nil {
			return util.Errorf("saving LAN share config: %w", err)
		}
	}

	return nil
}
