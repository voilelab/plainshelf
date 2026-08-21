// Command plainshelf-read is a standalone reading client for a PlainShelf
// shelf: point it at a shelf folder and it opens the library in the browser.
//
// It is the same server and the same frontend the full application ships, cut
// down to reading. There is no config file, no store beside the binary, and no
// route that writes: the shelf is opened read-only, so the folder it is given
// is left exactly as it was found - including the lock file, the temp
// directory and the exported book cache an ordinary server maintains.
//
// Reading progress and reading history still work; they are kept by the browser
// and never sent to the shelf.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// readerShelfID names the one shelf this binary serves. The frontend asks for a
// shelf by ID, and there is only ever this one, so it is a fixed word rather
// than something derived from the path - a path makes an unstable and
// unnecessarily revealing URL component.
const readerShelfID = "shelf"

// shutdownTimeout bounds how long a Ctrl-C waits for requests in flight. A
// reader serves a book's text and its illustrations, so a couple of seconds is
// generous.
const shutdownTimeout = 2 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "plainshelf-read:", err)
		os.Exit(1)
	}
}

func run() error {
	noBrowser := flag.Bool("no-browser", false,
		"print the address instead of opening a browser, for a machine that has none")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	shelfRoot, err := resolveShelfRoot(flag.Arg(0))
	if err != nil {
		return err
	}

	app, err := server.NewApp(readerConf(shelfRoot))
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer app.Close()

	if err := app.Start(); err != nil {
		return util.Errorf("%w", err)
	}

	// Port 0: the kernel picks a free one, so several copies can read several
	// shelves at once and none of them has a port to configure. Loopback only -
	// this serves a personal library with no authentication in front of it.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return util.Errorf("%w", err)
	}

	url := fmt.Sprintf("http://%s/", listener.Addr().String())
	fmt.Printf("PlainShelf %s reading %s\n", version.Version, shelfRoot)
	fmt.Printf("Open %s\nPress Ctrl-C to stop.\n", url)

	if !*noBrowser {
		// Advisory: a headless machine, a container or an SSH session is a
		// normal way to run this, and the address above is all such a user
		// needs. Reported rather than swallowed so a desktop failure is not
		// silent.
		if err := util.OpenBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "plainshelf-read:", err)
		}
	}

	return serve(&http.Server{Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second}, listener)
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage: %s [flags] <shelf-folder>\n\n", filepath.Base(os.Args[0]))
	fmt.Fprint(out, "Opens a PlainShelf shelf for reading. The folder is never written to.\n\nFlags:\n")
	flag.PrintDefaults()
}

// resolveShelfRoot turns the command line argument into the absolute path of an
// existing directory.
//
// Checked here rather than left to the shelf: a read-only shelf never creates
// its root, so a typo would otherwise surface as a scan failure in the log
// behind an empty library rather than as an answer to the command that was run.
func resolveShelfRoot(arg string) (string, error) {
	root, err := filepath.Abs(arg)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if !info.IsDir() {
		return "", util.Errorf("%s is not a folder", root)
	}

	return root, nil
}

// readerLogConf keeps warnings and errors on stderr and nothing else. The
// request log an ordinary server writes would be noise in a terminal the user
// is watching for an address, and there is nowhere to put a log file that would
// not be a write.
var readerLogConf = logutil.LogConf{Level: "warn", Format: "text"}

// readerConf is the whole configuration of this binary, written in code because
// there is nothing here for a user to choose.
//
// server.ServerModeReader is what makes it a reader: it mounts only the reading
// routes, implies read_only, and keeps the settings store in memory so nothing
// is written next to the binary either.
func readerConf(shelfRoot string) *server.AppConf {
	return &server.AppConf{
		Logger: readerLogConf,

		Mode:     server.ServerModeReader,
		ReadOnly: true,

		// The task pool is started but never fed - a reader mounts no route
		// that schedules a chain - so its logger only has a startup line to
		// print, and that line belongs nowhere near the address the user is
		// reading.
		Worker: &server.WorkerConf{Logger: readerLogConf},

		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:   readerShelfID,
				Name: filepath.Base(shelfRoot),
				ShelfConf: shelf.ShelfConf{
					Logger:   readerLogConf,
					LibRoot:  shelfRoot,
					ReadOnly: true,
				},
			},
		},

		// Loopback with no token: the same posture as the desktop app, which
		// also serves its own frontend to a browser on the same machine.
		Security: &server.SecurityConf{Mode: server.SecurityModeNone},
	}
}

// serve runs the server until the process is interrupted, then drains it.
func serve(srv *http.Server, listener net.Listener) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return util.Errorf("%w", err)
		}
		return nil
	case <-sigCh:
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
