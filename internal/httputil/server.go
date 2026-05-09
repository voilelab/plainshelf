package httputil

import (
	"net/http"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

type Conf struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

func NewServer(conf *Conf) (*http.Server, error) {
	readTimeout, err := time.ParseDuration(conf.ReadTimeout)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	writeTimeout, err := time.ParseDuration(conf.WriteTimeout)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &http.Server{
		Addr:         conf.Addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}, nil
}
