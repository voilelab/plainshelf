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

func startStuckServer(t *testing.T, release <-chan struct{}) (*http.Server, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
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
	return srv, served
}

func TestShutdownAllSharesOneDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	first, firstServed := startStuckServer(t, release)
	second, secondServed := startStuckServer(t, release)

	const timeout = 300 * time.Millisecond
	logger := &logutil.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	start := time.Now()
	shutdownAll(logger, timeout, namedServer{"first", first}, namedServer{"second", second})
	if elapsed := time.Since(start); elapsed >= 2*timeout {
		t.Fatalf("shutdown of two stuck servers took %v, want one shared %v deadline", elapsed, timeout)
	}
	for _, served := range []<-chan error{firstServed, secondServed} {
		if err := <-served; err != nil {
			t.Fatalf("serveUntilClosed = %v, want nil after shutdown", err)
		}
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
