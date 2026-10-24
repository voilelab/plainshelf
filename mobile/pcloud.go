package mobile

import (
	"encoding/json/v2"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

var pcloudHTTPClient *http.Client

// OpenPCloudShelf opens the shelf in a pCloud folder as a read-only virtual
// shelf under id, replacing one already open under that id. host is the
// account's region host and folder the shelf root's path in the account.
//
// Nothing is saved: the token stays in memory, so the caller passes it on
// every start from where it keeps it. scope is the cache scope key of the
// device's shelf entry; the offline copies are filed under it, so they follow
// the entry rather than id or name.
//
// showNSFW is the device's own adult-content choice, which the app keeps for
// pCloud apart from the setting local shelves use; SetShelfShowNSFW changes it.
func OpenPCloudShelf(id, name, host, token, folder, scope string, showNSFW bool) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return util.Errorf("shelf ID cannot be empty")
	}
	if scope == "" {
		return util.Errorf("virtual shelf scope cannot be empty")
	}
	client, err := pcloud.NewClient(host, token, pcloudHTTPClient)
	if err != nil {
		return util.Errorf("%w", err)
	}
	conf := shelf.ShelfConfWithID{ID: id, Name: name}
	root := pcloud.NewFS(client, folder)
	next := virtualShelf{
		scope: scope,
		conf:  conf,
		open:  func(app *server.App) error { return app.AddVirtualShelf(conf, root, scope) },
	}
	return rt.openVirtual(next, showNSFW)
}

func (rt *instance) closeVirtualForReplacement(id string) error {
	if err := rt.waitForAcceptedTaskChains(); err != nil {
		return err
	}
	if err := rt.app.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	delete(rt.virtual, id)
	return nil
}

func (rt *instance) restoreVirtual(previous virtualShelf) {
	if previous.open(rt.app) == nil {
		rt.virtual[previous.conf.ID] = previous
	}
}

func (rt *instance) openVirtual(next virtualShelf, showNSFW bool) error {
	id := next.conf.ID
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if _, err := rt.find(id); err == nil {
		return util.Errorf("shelf %q is a local shelf", id)
	}
	previous, replacing := rt.virtual[id]
	if replacing {
		if err := rt.closeVirtualForReplacement(id); err != nil {
			return err
		}
	}
	if err := next.open(rt.app); err != nil {
		if replacing {
			rt.restoreVirtual(previous)
		}
		return util.Errorf("%w", err)
	}
	rt.virtual[id] = next
	rt.app.SetShelfShowNSFW(id, showNSFW)
	return nil
}

// SetShelfShowNSFW changes the adult-content choice of a shelf OpenPCloudShelf
// or OpenSourceShelf opened.
func SetShelfShowNSFW(id string, show bool) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.rlockRunning(); err != nil {
		return err
	}
	defer rt.mu.RUnlock()
	if _, ok := rt.virtual[id]; !ok {
		return util.Errorf("virtual shelf %q is not open", id)
	}
	rt.app.SetShelfShowNSFW(id, show)
	return nil
}

type virtualShelf struct {
	scope string
	conf  shelf.ShelfConfWithID
	open  func(app *server.App) error
}

// CloseVirtualShelf closes a shelf OpenPCloudShelf or OpenSourceShelf opened. Its offline copies
// stay on the device.
func CloseVirtualShelf(id string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if _, ok := rt.virtual[id]; !ok {
		return util.Errorf("virtual shelf %q is not open", id)
	}
	if err := rt.waitForAcceptedTaskChains(); err != nil {
		return err
	}
	if err := rt.app.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	delete(rt.virtual, id)
	rt.app.ClearShelfShowNSFW(id)
	return nil
}

type nsfwFolderMark struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

type nsfwMarks struct {
	NSFW       bool            `json:"nsfw"`
	NSFWFolder *nsfwFolderMark `json:"nsfw_folder,omitzero"`
}

// ShelfNSFWMarks returns every book's adult-content marks on a shelf
// OpenPCloudShelf opened, whatever the device shows, as a JSON object keyed
// by book ID in the Book response's terms. The app writes them into downloads
// an older build stored without them, which it could not do from a listing
// that already hides the marked books.
func ShelfNSFWMarks(id string) (out string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	if err := rt.rlockRunning(); err != nil {
		return "", err
	}
	defer rt.mu.RUnlock()
	if _, ok := rt.virtual[id]; !ok {
		return "", util.Errorf("virtual shelf %q is not open", id)
	}
	shelfData, ok := rt.app.ShelfManager().GetShelf(id)
	if !ok {
		return "", util.Errorf("shelf %q not found", id)
	}
	listings, err := shelfData.ListBooksWithCharCount()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	marks := make(map[string]nsfwMarks, len(listings))
	for _, listing := range listings {
		meta := listing.Book.GetMeta()
		mark := nsfwMarks{NSFW: meta.NSFW}
		if rule, ok := shelfData.NSFWFolderRule(listing.Folders); ok {
			mark.NSFWFolder = &nsfwFolderMark{Path: rule.Path, Reason: rule.Reason}
		}
		marks[meta.ID] = mark
	}
	bs, err := json.Marshal(marks, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// RemoveVirtualShelfOffline deletes an entry's offline copies, even if it is
// not open. Switching entries and stopping the app must not call this.
func RemoveVirtualShelfOffline(scope string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if scope == "" {
		return util.NewError("virtual shelf scope cannot be empty")
	}
	if err := rt.waitForAcceptedTaskChains(); err != nil {
		return err
	}
	for id, opened := range rt.virtual {
		if opened.scope != scope {
			continue
		}
		if err := rt.app.RemoveShelf(id); err != nil {
			return err
		}
		delete(rt.virtual, id)
		rt.app.ClearShelfShowNSFW(id)
	}
	return rt.app.RemoveOfflineScope(scope)
}
