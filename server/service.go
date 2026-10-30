package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/voilelab/plainshelf/internal/httputil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"gopkg.in/yaml.v3"
)

type SrvConf struct {
	Logger     logutil.LogConf `yaml:"logger"`
	ServerConf *httputil.Conf  `yaml:"server_conf"`
	AppConf    *AppConf        `yaml:"app_conf"`
	DAVExport  *DAVExportConf  `yaml:"dav_export"`
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
	if conf.DAVExport != nil {
		if err := conf.DAVExport.validate(conf.ServerConf.Addr); err != nil {
			return nil, util.Errorf("%w", err)
		}
		if err := conf.DAVExport.checkTokenFileNotExported(conf.AppConf.Shelves); err != nil {
			return nil, util.Errorf("%w", err)
		}
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

	rootLogger.Info("PlainShelf", "version", version.Version)

	rootLogger.Info("Create App...")
	app, err := NewApp(conf.AppConf)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer app.Close()
	rootLogger.Info("Create App...done")

	app.SetInsecureNetworkWarning(app.security.InsecureNetworkExposure(conf.ServerConf.Addr))

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

	addr := server.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return util.Errorf("%w", err)
	}
	serveErr := make(chan error, 2)
	go func() {
		rootLogger.Info("Starting http server on", "addr", conf.ServerConf.Addr)
		serveErr <- serveUntilClosed(server, ln)
	}()

	servers := []namedServer{{"http server", server}}
	defer func() { shutdownAll(rootLogger, shutdownTimeout, servers...) }()

	if conf.DAVExport != nil {
		davServer, err := startDAVExport(conf.DAVExport, app, rootLogger, serveErr)
		if err != nil {
			return util.Errorf("%w", err)
		}
		servers = append(servers, namedServer{"dav export server", davServer})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case err := <-serveErr:
		return util.Errorf("%w", err)
	}

	return nil
}

func startDAVExport(conf *DAVExportConf, app *App, logger *logutil.Logger, serveErr chan<- error) (*http.Server, error) {
	if err := EnsureDAVExportToken(conf.TokenFile); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if davTokenFileExposed(conf.TokenFile) {
		logger.Warn("dav export token file is readable by other users; chmod 600 it", "token_file", conf.TokenFile)
	}
	server, err := httputil.NewServer(&httputil.Conf{
		Addr:         conf.Addr,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
	})
	if err != nil {
		return nil, util.Errorf("dav_export: %w", err)
	}
	server.Handler, err = app.davExportCORSHandler(conf.TokenFile, conf.AllowedOrigins)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	ln, err := net.Listen("tcp", conf.Addr)
	if err != nil {
		return nil, util.Errorf("dav_export: %w", err)
	}
	logger.Info("Starting dav export server on", "addr", conf.Addr, "token_file", conf.TokenFile)
	go func() {
		serveErr <- serveUntilClosed(server, ln)
	}()
	return server, nil
}

func serveUntilClosed(server *http.Server, ln net.Listener) error {
	err := server.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

const shutdownTimeout = 5 * time.Second

type namedServer struct {
	name   string
	server *http.Server
}

func shutdownAll(logger *logutil.Logger, timeout time.Duration, servers ...namedServer) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Go(func() {
			logger.Info("shutting down " + s.name)
			if err := s.server.Shutdown(ctx); err != nil {
				logger.Error("failed to shutdown "+s.name, "error", err)
				_ = s.server.Close()
			}
		})
	}
	wg.Wait()
}

func RotateDAVExportTokenForConf(confPath string) (token, tokenFile string, err error) {
	conf, err := loadAppConf(confPath)
	if err != nil {
		return "", "", util.Errorf("%w", err)
	}
	if conf.DAVExport == nil {
		return "", "", util.Errorf("config has no dav_export section")
	}
	tokenFile, err = filepath.Abs(conf.DAVExport.TokenFile)
	if err != nil {
		return "", "", util.Errorf("%w", err)
	}
	token, err = RotateDAVExportToken(tokenFile)
	return token, tokenFile, err
}
