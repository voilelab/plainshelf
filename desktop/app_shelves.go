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

	Source   string `json:"source"`
	URL      string `json:"url"`
	Username string `json:"username"`

	OpenError string `json:"open_error"`
}

type ShelfSourceParams struct {
	Source   string `json:"source"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

type AddShelfParams struct {
	Name              string `json:"name"`
	LibRoot           string `json:"libRoot"`
	ScanInterval      string `json:"scanInterval"`
	BookCheckInterval string `json:"bookCheckInterval"`
	ReadOnly          bool   `json:"readOnly"`

	Remote ShelfSourceParams `json:"remote"`
}

type ModifyShelfParams struct {
	ShelfID           string `json:"shelfID"`
	Name              string `json:"name"`
	ScanInterval      string `json:"scanInterval"`
	BookCheckInterval string `json:"bookCheckInterval"`
	ReadOnly          bool   `json:"readOnly"`
}

type ShelfIDPreview struct {
	ID string `json:"id"`

	DefaultPath string `json:"defaultPath"`
}

const virtualShelfOpenFailedSummary = "The virtual shelf could not be opened. Check the connection and unlock the keychain, then retry."

func (a *DesktopApp) PreviewShelfID(name string) (ShelfIDPreview, error) {
	if strings.TrimSpace(name) == "" {
		return ShelfIDPreview{}, nil
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return ShelfIDPreview{}, util.Errorf("loading shelf config: %w", err)
	}

	return a.previewFor(conf.nextShelfID(name)), nil
}

func (a *DesktopApp) previewFor(id string) ShelfIDPreview {
	return ShelfIDPreview{ID: id, DefaultPath: defaultDesktopShelfDir(a.shelvesConfigPath, id)}
}

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

	id := conf.nextShelfID(name)

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

func (a *DesktopApp) addVirtualShelf(name string, params AddShelfParams) error {
	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}
	id := conf.nextShelfID(name)

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
			details.OpenError = virtualShelfOpenFailedSummary
		}
	}
	return details
}

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
		return util.NewError(virtualShelfOpenFailedSummary)
	}
	return nil
}

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

func (a *DesktopApp) deleteShelfCredential(source *shelf.SourceConf) error {
	if source == nil || !strings.HasPrefix(source.Credential, hostCredentialPrefix) {
		return nil
	}
	if err := a.secretStore().Delete(strings.TrimPrefix(source.Credential, hostCredentialPrefix)); err != nil {
		return util.Errorf("removing the shelf's secret from the keychain: %w", err)
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
	source := removed.Source
	if err := a.deleteShelfCredential(source); err != nil {
		return err
	}

	isShelf := func(entry desktopShelfEntry) bool { return entry.ID == shelfID }
	conf.Shelves = slices.DeleteFunc(conf.Shelves, isShelf)
	conf.VirtualShelves = slices.DeleteFunc(conf.VirtualShelves, isShelf)

	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		return util.Errorf("saving shelf config: %w", err)
	}

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
