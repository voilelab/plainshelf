package shelf

import (
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/davfs"
	"github.com/voilelab/plainshelf/internal/util"
)

// Source types; see docs/concepts/shelf-sources.md.
const (
	SourceLocal      = "local"
	SourcePlainShelf = "plainshelf"
	SourceWebDAV     = "webdav"
	SourcePCloud     = "pcloud"
)

// SourceConf is where a shelf's files are and how they are reached. One flat
// struct for every type; Validate rejects the fields a type does not use.
type SourceConf struct {
	Type       string `yaml:"type" json:"type"`
	Path       string `yaml:"path" json:"path,omitzero"`
	URL        string `yaml:"url" json:"url,omitzero"`
	Host       string `yaml:"host" json:"host,omitzero"`
	UserID     string `yaml:"user_id" json:"user_id,omitzero"`
	ClientID   string `yaml:"client_id" json:"client_id,omitzero"`
	Username   string `yaml:"username" json:"username,omitzero"`
	Credential string `yaml:"credential" json:"credential,omitzero"`
}

// credentialSchemes are the references a credential may hold; never the
// secret itself.
var credentialSchemes = []string{"env:", "file:", "host:"}

// Validate checks the fields the type needs and refuses every field it does
// not use, so a typo cannot silently open the wrong thing.
func (c *SourceConf) Validate() error {
	var allowed, required []string
	switch c.Type {
	case SourceLocal:
		allowed, required = []string{"path"}, []string{"path"}
	case SourcePlainShelf:
		allowed, required = []string{"url", "credential"}, []string{"url", "credential"}
	case SourceWebDAV:
		allowed, required = []string{"url", "username", "credential"}, []string{"url"}
	case SourcePCloud:
		allowed, required = []string{"host", "user_id", "client_id", "path", "credential"}, []string{"host", "path", "credential"}
	default:
		return util.Errorf("unknown source type %q: must be local, plainshelf, webdav or pcloud", c.Type)
	}

	fields := map[string]string{
		"path": c.Path, "url": c.URL, "host": c.Host, "user_id": c.UserID,
		"client_id": c.ClientID, "username": c.Username, "credential": c.Credential,
	}
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if fields[key] != "" && !slices.Contains(allowed, key) {
			return util.Errorf("source type %q does not take %q", c.Type, key)
		}
	}
	for _, key := range required {
		if fields[key] == "" {
			return util.Errorf("source type %q requires %q", c.Type, key)
		}
	}
	if c.Username != "" && c.Credential == "" {
		return util.Errorf("source %q has a username but no credential", c.Type)
	}

	if c.Credential != "" {
		if err := validateCredentialRef(c.Credential); err != nil {
			return err
		}
	}
	if c.URL != "" {
		if _, err := davfs.ParseURL(c.URL); err != nil {
			return util.Errorf("%w", err)
		}
	}
	return nil
}

// validateCredentialRef refuses anything but a reference, a pasted token
// included: a configuration file ends up in bug reports and backups.
func validateCredentialRef(ref string) error {
	for _, scheme := range credentialSchemes {
		if value, ok := strings.CutPrefix(ref, scheme); ok {
			if strings.TrimSpace(value) == "" {
				return util.Errorf("credential reference %q names nothing", scheme)
			}
			return nil
		}
	}
	return util.NewError("credential must be a reference (env:NAME, file:/path or host:ID), never the secret itself")
}

// IsVirtual reports a source read through a remote ReadFS rather than a local
// path.
func (c *SourceConf) IsVirtual() bool {
	return c.Type != SourceLocal
}

// Scope names a virtual source by where it lives, so renaming the shelf keeps
// what a device stores for it; see docs/concepts/shelf-sources.md#cache-scope.
// It is empty for a local source.
func (c *SourceConf) Scope() string {
	switch c.Type {
	case SourcePlainShelf, SourceWebDAV:
		u, err := davfs.ParseURL(c.URL)
		if err != nil {
			return ""
		}
		if c.Username != "" {
			u.User = url.User(c.Username)
		}
		return u.String()
	case SourcePCloud:
		scope := "pcloud://" + c.Host
		if c.UserID != "" {
			scope += "/" + c.UserID
		}
		return scope + "/" + strings.TrimPrefix(c.Path, "/")
	}
	return ""
}

// ResolveSource folds lib_root into a local source and validates the result.
// lib_root and source are mutually exclusive, and one of them is required.
func (c *ShelfConf) ResolveSource() (SourceConf, error) {
	switch {
	case c.Source != nil && c.LibRoot != "":
		return SourceConf{}, util.NewError("lib_root and source are mutually exclusive")
	case c.Source == nil && c.LibRoot == "":
		return SourceConf{}, util.NewError("lib_root or source is required")
	case c.Source == nil:
		return SourceConf{Type: SourceLocal, Path: c.LibRoot}, nil
	}
	src := *c.Source
	if err := src.Validate(); err != nil {
		return SourceConf{}, err
	}
	return src, nil
}
