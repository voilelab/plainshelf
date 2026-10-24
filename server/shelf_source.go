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

type CredentialResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

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

func openSourceFS(conf *shelf.ShelfConf, resolver CredentialResolver) (root fsutil.ReadFS, scope string, err error) {
	src, err := conf.ResolveSource()
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	switch src.Type {
	case shelf.SourceLocal:
		return nil, "", nil
	case shelf.SourcePCloud:
		return nil, "", util.Errorf("source type %q is not available on this client", src.Type)
	}

	if resolver == nil {
		resolver = DefaultCredentialResolver{}
	}
	opts := davfs.Options{Username: src.Username}
	if src.Credential != "" {
		secret, err := resolvableSecret(resolver, src.Credential)
		if err != nil {
			return nil, "", util.Errorf("%w", err)
		}
		opts.Secret = secret
	}
	dav, err := davfs.New(src.URL, opts)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	return dav, src.Scope(), nil
}

func resolvableSecret(resolver CredentialResolver, ref string) (func(context.Context) (string, error), error) {
	if _, err := resolver.Resolve(context.Background(), ref); err != nil {
		return nil, err
	}
	return func(ctx context.Context) (string, error) { return resolver.Resolve(ctx, ref) }, nil
}

func addShelfEntry(sm *shelf.ShelfManager, conf shelf.ShelfConfWithID, storePath string, resolver CredentialResolver) error {
	root, scope, err := openSourceFS(&conf.ShelfConf, resolver)
	if err != nil {
		return util.Errorf("shelf %q: %w", conf.ID, err)
	}
	if root == nil {
		return sm.AddShelf(conf)
	}

	conf.ReadOnly = true
	conf.BookCacheWriterID = ""
	return sm.AddVirtualShelf(conf, root, offlineDir(storePath, scope))
}

const (
	SourceCheckUnauthorized = "SOURCE_UNAUTHORIZED"
	SourceCheckUnavailable  = "SOURCE_UNAVAILABLE"
	SourceCheckNotAShelf    = "NOT_A_SHELF"
)

const probeCredentialRef = "host:check"

type SourceCheck struct {
	Code  string `json:"code,omitzero"`
	Books int    `json:"books"`
}

func CheckSource(sourceType, rawURL, username, secret string) (SourceCheck, error) {
	src := probeSourceConf(sourceType, rawURL, username, secret != "")
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

func probeSourceConf(sourceType, rawURL, username string, hasSecret bool) shelf.SourceConf {
	src := shelf.SourceConf{Type: sourceType, URL: rawURL, Username: username}
	if hasSecret {
		src.Credential = probeCredentialRef
	}
	return src
}
