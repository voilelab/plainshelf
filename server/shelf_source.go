package server

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/voilelab/plainshelf/internal/davfs"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// CredentialResolver turns a credential reference (env:NAME, file:/path,
// host:ID) into the secret it names. The secret is held only by the source
// client; it never reaches a log, the store, or the HTTP API.
type CredentialResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// DefaultCredentialResolver handles env: and file:, and refuses host:, which
// only a host with a secret store (desktop, Android, wasm) can answer.
type DefaultCredentialResolver struct{}

func (DefaultCredentialResolver) Resolve(_ context.Context, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimPrefix(ref, "env:")
		value, ok := os.LookupEnv(name)
		if !ok || strings.TrimSpace(value) == "" {
			return "", util.Errorf("credential %q: environment variable is not set", ref)
		}
		return strings.TrimSpace(value), nil
	case strings.HasPrefix(ref, "file:"):
		bs, err := os.ReadFile(strings.TrimPrefix(ref, "file:"))
		if err != nil {
			return "", util.Errorf("credential %q: %w", ref, err)
		}
		value := strings.TrimSpace(string(bs))
		if value == "" {
			return "", util.Errorf("credential %q: file is empty", ref)
		}
		return value, nil
	}
	return "", util.Errorf("credential %q: this host has no secret store for that scheme", ref)
}

// openSourceFS builds the ReadFS a virtual source is read through and the
// scope its offline copies are filed under. root is nil for a local source,
// which NewShelf opens itself.
//
// The credential is resolved once here, so a reference that names nothing
// fails startup instead of every later request.
func openSourceFS(conf *shelf.ShelfConf, resolver CredentialResolver) (root fsutil.ReadFS, scope string, err error) {
	src, err := conf.ResolveSource()
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	switch src.Type {
	case shelf.SourceLocal:
		return nil, "", nil
	case shelf.SourcePCloud:
		// CLAUDE.md limits pCloud to the Android client for now.
		return nil, "", util.Errorf("source type %q is not available on this client", src.Type)
	}

	if resolver == nil {
		resolver = DefaultCredentialResolver{}
	}
	opts := davfs.Options{Username: src.Username}
	if src.Credential != "" {
		ref := src.Credential
		if _, err := resolver.Resolve(context.Background(), ref); err != nil {
			return nil, "", util.Errorf("%w", err)
		}
		opts.Secret = func(ctx context.Context) (string, error) { return resolver.Resolve(ctx, ref) }
	}
	dav, err := davfs.New(src.URL, opts)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	return dav, src.Scope(), nil
}

// addShelfEntry opens one configured shelf, routing a virtual source to
// AddVirtualShelf with its offline copies under store_path.
func addShelfEntry(sm *shelf.ShelfManager, conf shelf.ShelfConfWithID, storePath string, resolver CredentialResolver) error {
	root, scope, err := openSourceFS(&conf.ShelfConf, resolver)
	if err != nil {
		return util.Errorf("shelf %q: %w", conf.ID, err)
	}
	if root == nil {
		return sm.AddShelf(conf)
	}
	// A virtual shelf exports nothing and writes nothing.
	conf.ReadOnly = true
	conf.BookCacheWriterID = ""
	return sm.AddVirtualShelf(conf, root, offlineDir(storePath, scope))
}

// Source check codes, the API's error codes for the same failures.
const (
	SourceCheckUnauthorized = "SOURCE_UNAUTHORIZED"
	SourceCheckUnavailable  = "SOURCE_UNAVAILABLE"
	SourceCheckNotAShelf    = "NOT_A_SHELF"
)

// SourceCheck is what CheckSource found. Code is empty when the source is a
// shelf; Books is then its book count.
type SourceCheck struct {
	Code  string `json:"code,omitzero"`
	Books int    `json:"books"`
}

// CheckSource probes a plainshelf or webdav source with a secret the user has
// typed but not yet stored, so a host's add-shelf form can tell a wrong token
// from no connection before saving. secret may be empty for a webdav server
// that needs none. It is a host call, never an HTTP route.
func CheckSource(sourceType, rawURL, username, secret string) (SourceCheck, error) {
	src := shelf.SourceConf{Type: sourceType, URL: rawURL, Username: username}
	if secret != "" {
		// Validate checks the reference's shape only; the secret is passed below.
		src.Credential = "host:check"
	}
	if src.Type != shelf.SourcePlainShelf && src.Type != shelf.SourceWebDAV {
		return SourceCheck{}, util.Errorf("source type %q cannot be checked here", src.Type)
	}
	if err := src.Validate(); err != nil {
		return SourceCheck{}, util.Errorf("%w", err)
	}
	opts := davfs.Options{Username: username}
	if secret != "" {
		opts.Secret = func(context.Context) (string, error) { return secret, nil }
	}
	dav, err := davfs.New(src.URL, opts)
	if err != nil {
		return SourceCheck{}, util.Errorf("%w", err)
	}
	books, err := shelf.CountBookPackages(dav)
	switch {
	case err == nil:
		return SourceCheck{Books: books}, nil
	case errors.Is(err, shelf.ErrNotAShelf), errors.Is(err, fs.ErrNotExist):
		return SourceCheck{Code: SourceCheckNotAShelf}, nil
	case errors.Is(err, fsutil.ErrSourceUnauthorized):
		return SourceCheck{Code: SourceCheckUnauthorized}, nil
	case errors.Is(err, fsutil.ErrSourceUnavailable):
		return SourceCheck{Code: SourceCheckUnavailable}, nil
	}
	return SourceCheck{}, util.Errorf("%w", err)
}
