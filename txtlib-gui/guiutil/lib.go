package guiutil

import (
	"encoding/json"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
)

type LibType string

const (
	LibTypeURI    LibType = "uri"
	LibTypeWebDAV LibType = "webdav"
)

type LibConf struct {
	// Type is the source type of library, e.g. "uri", "webdav"
	Type LibType `json:"type"`
	Conf any     `json:"conf"`
}

type LibConfURI struct {
	URI string `json:"uri"`
}

type LibConfWebDAV struct {
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user"`
	Password string `json:"password"`
	BaseDir  string `json:"baseDir"`
}

func parseLibConfConf[T any](conf any) (T, error) {
	var zero T
	bs, err := json.Marshal(conf)
	if err != nil {
		return zero, util.Errorf("failed to marshal library config: %w", err)
	}

	var typedConf T
	err = json.Unmarshal(bs, &typedConf)
	if err != nil {
		return zero, util.Errorf("failed to unmarshal library config: %w", err)
	}

	return typedConf, nil
}

func NewLib(conf *LibConf) (*txtlib.Txtlib, error) {
	switch conf.Type {
	case LibTypeURI:
		uriConf, err := parseLibConfConf[LibConfURI](conf.Conf)
		if err != nil {
			return nil, util.Errorf("invalid config for URI library: %w", err)
		}
		folder, err := ParseListableURI(uriConf.URI)
		if err != nil {
			return nil, util.Errorf("failed to parse library URI: %w", err)
		}
		return txtlib.OpenLib(fsutil.NewFyneURIFS(folder), false)

	case LibTypeWebDAV:
		webdavConf, err := parseLibConfConf[LibConfWebDAV](conf.Conf)
		if err != nil {
			return nil, util.Errorf("invalid config for WebDAV library: %w", err)
		}
		fs, err := fsutil.NewWebDAVFS(&fsutil.WebDAVConf{
			Host:     webdavConf.Host,
			Port:     webdavConf.Port,
			User:     webdavConf.User,
			Password: webdavConf.Password,
			BaseDir:  webdavConf.BaseDir,
		})
		if err != nil {
			return nil, util.Errorf("failed to create WebDAV FS: %w", err)
		}
		return txtlib.OpenLib(fs, false)

	default:
		return nil, util.Errorf("unsupported library type: %s", conf.Type)
	}
}
