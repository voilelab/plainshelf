package guiutil

import (
	"github.com/voilelab/plainshelf/internal/util"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// ParseListableURI parses a raw URI string and returns a ListableURI.
func ParseListableURI(raw string) (fyne.ListableURI, error) {
	uri, err := storage.ParseURI(raw)
	if err != nil {
		return nil, util.Errorf("invalid library URI: %w", err)
	}

	folder, err := storage.ListerForURI(uri)
	if err != nil {
		return nil, util.Errorf("cannot access library URI: %w", err)
	}

	return folder, nil
}

// DisplayURI returns a human-readable string for a URI, preferring the path,
// then the name, then the full URI string.
func DisplayURI(uri fyne.URI) string {
	if uri == nil {
		return ""
	}

	if p := uri.Path(); p != "" {
		return p
	}

	if name := uri.Name(); name != "" {
		return name
	}

	return uri.String()
}

// LocalPathFromURI returns the local filesystem path for a file:// URI,
// or an empty string for any other scheme.
func LocalPathFromURI(uri fyne.URI) string {
	if uri == nil || uri.Scheme() != "file" {
		return ""
	}
	return uri.Path()
}
