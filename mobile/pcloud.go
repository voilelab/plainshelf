package mobile

import (
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// pcloudHTTPClient is nil outside tests, which route requests to a fake.
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
	if scope == "" {
		return util.Errorf("virtual shelf scope cannot be empty")
	}
	client, err := pcloud.NewClient(host, token, pcloudHTTPClient)
	if err != nil {
		return util.Errorf("%w", err)
	}
	next := virtualShelf{
		conf:  shelf.ShelfConfWithID{ID: id, Name: name},
		root:  pcloud.NewFS(client, folder),
		scope: scope,
	}

	if err := rt.lock(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if _, err := rt.find(id); err == nil {
		return util.Errorf("shelf %q is a local shelf", id)
	}
	previous, replacing := rt.virtual[id]
	if replacing {
		// A chain accepted earlier may still read the shelf being replaced.
		if err := rt.quiesce(); err != nil {
			return err
		}
		if err := rt.app.RemoveShelf(id); err != nil {
			return util.Errorf("%w", err)
		}
		delete(rt.virtual, id)
	}
	if err := rt.app.AddVirtualShelf(next.conf, next.root, next.scope); err != nil {
		// Put back the shelf that was working.
		if replacing && rt.app.AddVirtualShelf(previous.conf, previous.root, previous.scope) == nil {
			rt.virtual[id] = previous
		}
		return util.Errorf("%w", err)
	}
	rt.virtual[id] = next
	return nil
}

// virtualShelf is what reopens a virtual shelf after a failed replacement.
type virtualShelf struct {
	conf  shelf.ShelfConfWithID
	root  fsutil.ReadFS
	scope string
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
	if _, ok := rt.virtual[id]; !ok {
		return util.Errorf("virtual shelf %q is not open", id)
	}
	if err := rt.quiesce(); err != nil {
		return err
	}
	if err := rt.app.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	delete(rt.virtual, id)
	return nil
}
