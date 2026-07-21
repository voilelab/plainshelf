// Package version exposes the application version as a single source of truth.
package version

// Version is the full application build version derived from Git tags. It
// defaults to "dev" and is overridden at build time via the linker, e.g.:
//
//	go build -ldflags "-X github.com/voilelab/plainshelf/internal/version.Version=v1.0.0"
var Version = "dev"
