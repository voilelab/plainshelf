package server

import (
	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

type WorkerConf struct {
	Logger logutil.LogConf `yaml:"logger"`
	MaxLen int             `yaml:"max_len"`

	// MaxKeep bounds how many finished task chains stay queryable through the
	// task chain API. Zero selects the package default.
	MaxKeep int `yaml:"max_keep"`
}

// ServerMode names how much of the HTTP surface a server mounts.
//
// It is not a permission setting - read_only is what refuses writes, and it
// stays the thing to reach for on an ordinary server. This decides which routes
// exist at all, which is what lets one binary be a reading client rather than a
// library server that happens to be read-only.
type ServerMode string

const (
	// ServerModeFull mounts the whole API. The zero value, so a config that
	// says nothing gets the server it has always got.
	ServerModeFull ServerMode = ""

	// ServerModeReader mounts only the GET routes a reader needs: the shelf,
	// its books, their covers, content, sources and layers. The log, setting,
	// task, trash and duplicate-scan surfaces are not part of a reading client,
	// so they are not served.
	//
	// Implies read_only; see AppConf.readOnly.
	ServerModeReader ServerMode = "reader"
)

func (m ServerMode) valid() bool {
	switch m {
	case ServerModeFull, ServerModeReader:
		return true
	default:
		return false
	}
}

type AppConf struct {
	Logger             logutil.LogConf          `yaml:"logger"`
	Shelves            []*shelf.ShelfConfWithID `yaml:"shelves"`
	Worker             *WorkerConf              `yaml:"worker"`
	StorePath          string                   `yaml:"store_path"`
	CoverToJPG         bool                     `yaml:"cover_to_jpg"`
	EPUBImportStrategy *epub.Strategy           `yaml:"epub_import_strategy"`
	ReadOnly           bool                     `yaml:"read_only"`
	Security           *SecurityConf            `yaml:"security"`

	// Mode selects the HTTP surface; see ServerMode. Empty means the full API.
	Mode ServerMode `yaml:"mode"`
}

// readOnly reports whether this server may write anything at all.
//
// Reader mode implies it rather than being validated against it: a reading
// binary that wrote to the shelf because a caller forgot a second flag would
// corrupt the one promise it makes. Every read of AppConf.ReadOnly goes through
// here so the implication cannot be missed at one of them.
func (c *AppConf) readOnly() bool {
	return c.ReadOnly || c.Mode == ServerModeReader
}
