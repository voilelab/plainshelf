package mobile

import (
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// pcloudHTTPClient is nil outside tests, which swap in one that trusts a fake.
var pcloudHTTPClient *http.Client

// OpenPCloudShelf opens the shelf in a pCloud folder as a read-only virtual
// shelf under id, replacing one already open under that id. host is the
// account's region host and folder the shelf root's path in the account.
//
// Nothing is saved: the token stays in memory, so the caller passes it on
// every start from where it keeps it. scope is the cache scope key of the
// device's shelf entry; the offline copies are filed under it, so they follow
// the entry rather than id or name.
func OpenPCloudShelf(id, name, host, token, folder, scope string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return util.Errorf("shelf ID cannot be empty")
	}
	client, err := pcloud.NewClient(host, token, pcloudHTTPClient)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := rt.lock(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if _, err := rt.find(id); err == nil {
		return util.Errorf("shelf %q is a local shelf", id)
	}
	if rt.virtual[id] {
		if err := rt.app.RemoveShelf(id); err != nil {
			return util.Errorf("%w", err)
		}
		delete(rt.virtual, id)
	}
	conf := shelf.ShelfConfWithID{ID: id, Name: name}
	if err := rt.app.AddVirtualShelf(conf, pcloud.NewFS(client, folder), scope); err != nil {
		return util.Errorf("%w", err)
	}
	rt.virtual[id] = true
	return nil
}

// CloseVirtualShelf closes a shelf OpenPCloudShelf opened. Its offline copies
// stay on the device.
func CloseVirtualShelf(id string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lock(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if !rt.virtual[id] {
		return util.Errorf("virtual shelf %q is not open", id)
	}
	if err := rt.app.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	delete(rt.virtual, id)
	return nil
}
