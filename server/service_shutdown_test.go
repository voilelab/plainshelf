package server

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
)

func TestShutdownWithinStuckHandler(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	srv := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	})}
	served := make(chan error, 1)
	go func() { served <- serveUntilClosed(srv, ln) }()
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered

	logger := &logutil.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	start := time.Now()
	shutdownWithin(srv, "test server", logger, 50*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("shutdown took %v with a stuck handler, want about the timeout", elapsed)
	}
	if err := <-served; err != nil {
		t.Fatalf("serveUntilClosed = %v, want nil after shutdown", err)
	}
}

func TestServeUntilClosedListenerError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	if err := serveUntilClosed(&http.Server{}, ln); err == nil {
		t.Fatal("serveUntilClosed on a closed listener = nil, want the accept error")
	}
}
