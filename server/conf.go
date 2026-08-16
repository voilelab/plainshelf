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

type AppConf struct {
	Logger             logutil.LogConf          `yaml:"logger"`
	Shelves            []*shelf.ShelfConfWithID `yaml:"shelves"`
	Worker             *WorkerConf              `yaml:"worker"`
	StorePath          string                   `yaml:"store_path"`
	CoverToJPG         bool                     `yaml:"cover_to_jpg"`
	DefaultSplitConfig *shelf.SplitConfig       `yaml:"default_split_config"`
	EPUBImportStrategy *epub.Strategy           `yaml:"epub_import_strategy"`
	ReadOnly           bool                     `yaml:"read_only"`
	Security           *SecurityConf            `yaml:"security"`
}
