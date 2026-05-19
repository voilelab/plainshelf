package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/voilelab/plainshelf/server"
)

const (
	defaultAddr             = "127.0.0.1:0"
	defaultReadHistoryLimit = 100
	shutdownTimeout         = 10 * time.Second
)

type readyEvent struct {
	Type        string `json:"type"`
	BaseURL     string `json:"base_url"`
	Addr        string `json:"addr"`
	Token       string `json:"token"`
	TokenHeader string `json:"token_header"`
	ProfileDir  string `json:"profile_dir"`
	ShelfPath   string `json:"shelf_path"`
	StorePath   string `json:"store_path"`
	PID         int    `json:"pid"`
}

type errorEvent struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("plainshelf-gui-sidecar: ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := run(); err != nil {
		writeJSON(errorEvent{Type: "error", Error: err.Error()})
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	var addr string
	var profileDir string
	var readHistoryLimit int
	var coverToJPG bool

	flag.StringVar(&addr, "addr", defaultAddr, "loopback address for the GUI sidecar HTTP server")
	flag.StringVar(&profileDir, "profile", "", "PlainShelf desktop profile directory")
	flag.IntVar(&readHistoryLimit, "read-history-limit", defaultReadHistoryLimit, "maximum read history entries")
	flag.BoolVar(&coverToJPG, "cover-to-jpg", true, "convert uploaded covers to JPEG")
	flag.Parse()

	resolvedProfileDir, err := resolveProfileDir(profileDir)
	if err != nil {
		return err
	}
	if err := ensureProfileLayout(resolvedProfileDir); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %q: %w", addr, err)
	}
	defer ln.Close()

	actualAddr := ln.Addr().String()
	baseURL := loopbackBaseURL(actualAddr)

	allowMissingOriginWithToken := true
	appConf := &server.AppConf{
		ShelfPath:        filepath.Join(resolvedProfileDir, "shelf"),
		StorePath:        filepath.Join(resolvedProfileDir, "store"),
		CoverToJPG:       coverToJPG,
		ReadHistoryLimit: readHistoryLimit,
		Security: &server.SecurityConf{
			Mode:                        server.SecurityModeLocalToken,
			ProtectRead:                 true,
			TokenHeader:                 "X-PlainShelf-Token",
			AllowMissingOriginWithToken: &allowMissingOriginWithToken,
			AllowedOrigins: []string{
				baseURL,
				strings.Replace(baseURL, "127.0.0.1", "localhost", 1),
			},
		},
	}

	app, err := server.NewApp(appConf)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}
	defer app.Close()

	if err := app.Start(); err != nil {
		return fmt.Errorf("start app: %w", err)
	}

	httpServer := &http.Server{
		Handler:      app.Handler(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Println("serving GUI sidecar on", actualAddr)
		serveErr <- httpServer.Serve(ln)
	}()

	writeJSON(readyEvent{
		Type:        "ready",
		BaseURL:     baseURL,
		Addr:        actualAddr,
		Token:       app.SecurityToken(),
		TokenHeader: app.SecurityTokenHeader(),
		ProfileDir:  resolvedProfileDir,
		ShelfPath:   appConf.ShelfPath,
		StorePath:   appConf.StorePath,
		PID:         os.Getpid(),
	})

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownCh)

	stdinClosed := make(chan struct{})
	go waitForStdinClose(stdinClosed)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve sidecar: %w", err)
		}
		return nil
	case sig := <-shutdownCh:
		log.Println("received shutdown signal", sig)
	case <-stdinClosed:
		log.Println("stdin closed; shutting down sidecar")
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown sidecar HTTP server: %w", err)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve sidecar: %w", err)
	}
	return nil
}

func writeJSON(v any) {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(v); err != nil {
		log.Println("failed to write JSON event:", err)
	}
}

func waitForStdinClose(done chan<- struct{}) {
	defer close(done)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "shutdown" {
			return
		}
	}
}

func resolveProfileDir(profileDir string) (string, error) {
	if strings.TrimSpace(profileDir) != "" {
		return filepath.Abs(profileDir)
	}

	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "PlainShelf"), nil
	case "windows":
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			return filepath.Join(appData, "PlainShelf"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming", "PlainShelf"), nil
	default:
		if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "plainshelf"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "plainshelf"), nil
	}
}

func ensureProfileLayout(profileDir string) error {
	for _, dir := range []string{
		profileDir,
		filepath.Join(profileDir, "shelf"),
		filepath.Join(profileDir, "store"),
		filepath.Join(profileDir, "logs"),
		filepath.Join(profileDir, "backups"),
		filepath.Join(profileDir, "tmp"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create profile directory %q: %w", dir, err)
		}
	}
	return nil
}

func loopbackBaseURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::1" {
		return "http://[::1]:" + port
	}
	return "http://" + host + ":" + port
}
