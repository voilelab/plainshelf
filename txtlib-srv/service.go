package txtlibsrv

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/voilelab/plainshelf/internal/httputil"
	"github.com/voilelab/plainshelf/internal/util"
	"gopkg.in/yaml.v3"
)

type SrvConf struct {
	ServerConf *httputil.Conf `yaml:"server_conf"`
	AppConf    *AppConf       `yaml:"app_conf"`
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

	return &conf, nil
}

func MainService(confPath string) error {
	conf, err := loadAppConf(confPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	log.Println("Create App...")
	app, err := NewApp(conf.AppConf)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer app.Close()
	log.Println("Create App...done")

	log.Println("Start App...")
	err = app.Start()
	if err != nil {
		return util.Errorf("%w", err)
	}
	log.Println("Start App...done")

	mux := http.NewServeMux()
	app.Serve(mux)

	server, err := httputil.NewServer(conf.ServerConf)
	if err != nil {
		return util.Errorf("%w", err)
	}
	server.Handler = mux

	go func() {
		log.Println("Starting http server on", conf.ServerConf.Addr)
		err = server.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	}()

	defer func() {
		log.Println("shutting down http server")
		err = server.Shutdown(context.TODO())
		if err != nil {
			log.Println("failed to shutdown http server:", err)
		}
	}()

	// listen sigterm and sigint, and gracefully shutdown the server
	// prevent sig
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return nil
}
