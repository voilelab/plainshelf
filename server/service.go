package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/voilelab/plainshelf/internal/httputil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"gopkg.in/yaml.v3"
)

type SrvConf struct {
	Logger     logutil.LogConf `yaml:"logger"`
	ServerConf *httputil.Conf  `yaml:"server_conf"`
	AppConf    *AppConf        `yaml:"app_conf"`
}

func loadAppConf(confPath string) (*SrvConf, error) {
	bs, err := os.ReadFile(confPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var conf SrvConf
	err = yaml.Unmarshal(bs, &conf)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if conf.AppConf == nil {
		return nil, util.Errorf("invalid config: missing app_conf")
	}
	if conf.ServerConf == nil {
		return nil, util.Errorf("invalid config: missing server_conf")
	}

	if err := ValidateSecurityForListenAddr(conf.AppConf.Security, conf.ServerConf.Addr); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &conf, nil
}

func MainService(confPath string) error {
	conf, err := loadAppConf(confPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	rootLogger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer func() {
		if err := rootLogger.Close(); err != nil {
			_, _ = os.Stderr.WriteString("failed to close root logger: " + err.Error() + "\n")
		}
	}()

	rootLogger.Info("Create App...")
	app, err := NewApp(conf.AppConf)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer app.Close()
	rootLogger.Info("Create App...done")

	rootLogger.Info("Start App...")
	err = app.Start()
	if err != nil {
		return util.Errorf("%w", err)
	}
	rootLogger.Info("Start App...done")

	app.security.LogStartup(rootLogger)

	server, err := httputil.NewServer(conf.ServerConf)
	if err != nil {
		return util.Errorf("%w", err)
	}
	server.Handler = app.Handler()

	go func() {
		rootLogger.Info("Starting http server on", "addr", conf.ServerConf.Addr)
		err = server.ListenAndServe()
		if err != nil {
			rootLogger.Error("http server error", "error", err)
		}
	}()

	defer func() {
		rootLogger.Info("shutting down http server")
		err = server.Shutdown(context.TODO())
		if err != nil {
			rootLogger.Error("failed to shutdown http server", "error", err)
		}
	}()

	// listen sigterm and sigint, and gracefully shutdown the server
	// prevent sig
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return nil
}
