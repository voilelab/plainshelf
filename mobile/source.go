package mobile

import (
	"context"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// SecretStore is the app's secret storage (Android Keystore through the
// SecureStorage plugin), implemented natively. Secret returns what is stored
// for a shelf entry ID, or "" when nothing is.
type SecretStore interface {
	Secret(entryID string) (string, error)
}

var (
	secretsMu sync.Mutex
	secrets   SecretStore
)

// SetSecretStore installs the native secret store that answers host:
// credentials. Called once, before Start.
func SetSecretStore(store SecretStore) {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	secrets = store
}

func storedSecret(entryID string) (string, error) {
	secretsMu.Lock()
	store := secrets
	secretsMu.Unlock()
	if store == nil {
		return "", util.NewError("no secret store is installed")
	}
	secret, err := store.Secret(entryID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return strings.TrimSpace(secret), nil
}

// credentialResolver answers host:<entryID> from the native store and hands
// every other scheme to the server's default.
type credentialResolver struct{}

func (credentialResolver) Resolve(ctx context.Context, ref string) (string, error) {
	id, ok := strings.CutPrefix(ref, "host:")
	if !ok {
		return server.DefaultCredentialResolver{}.Resolve(ctx, ref)
	}
	secret, err := storedSecret(id)
	if err != nil {
		return "", util.Errorf("credential %q: %w", ref, err)
	}
	if secret == "" {
		return "", util.Errorf("credential %q: no secret is stored for this shelf", ref)
	}
	return secret, nil
}

// OpenSourceShelf opens a plainshelf or webdav source as a read-only virtual
// shelf under id, replacing one already open under that id. Its credential is
// host:<entryID> when the native store holds a secret for the entry, so the
// secret never crosses the bridge; offline copies are filed under the
// source's own scope.
func OpenSourceShelf(id, name, sourceType, rawURL, username, entryID string, showNSFW bool) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return util.Errorf("shelf ID cannot be empty")
	}
	if sourceType != shelf.SourcePlainShelf && sourceType != shelf.SourceWebDAV {
		return util.Errorf("source type %q is not available here", sourceType)
	}
	src := &shelf.SourceConf{Type: sourceType, URL: rawURL, Username: username}
	secret, err := storedSecret(entryID)
	if err != nil {
		return err
	}
	if secret != "" {
		src.Credential = "host:" + entryID
	}
	conf := shelf.ShelfConfWithID{ID: id, Name: name, ShelfConf: shelf.ShelfConf{Source: src}}
	next := virtualShelf{
		scope: src.Scope(),
		conf:  conf,
		open:  func(app *server.App) error { return app.AddShelf(conf) },
	}
	return rt.openVirtual(next, showNSFW)
}

// CheckSource probes a source with the secret the setup form holds, before an
// entry stores it, as JSON {"code", "books"}; see server.CheckSource. It needs
// no running server.
func CheckSource(sourceType, rawURL, username, secret string) (out string, err error) {
	defer recoverInto(&err)
	check, err := server.CheckSource(sourceType, rawURL, username, secret)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return marshalJSON(check)
}

// RemoveSourceShelfOffline derives the same scope used by OpenSourceShelf,
// without resolving credentials or connecting to the source being removed.
func RemoveSourceShelfOffline(sourceType, rawURL, username string) error {
	if sourceType != shelf.SourcePlainShelf && sourceType != shelf.SourceWebDAV {
		return util.Errorf("source type %q is not available here", sourceType)
	}
	src := &shelf.SourceConf{Type: sourceType, URL: rawURL, Username: username}
	return RemoveVirtualShelfOffline(src.Scope())
}
