package util

import (
	"os/exec"
	"runtime"
)

func OpenFinder(path string) error {
	if err := openWithHostHandler(path); err != nil {
		return Errorf("failed to open file explorer: %w", err)
	}
	return nil
}

// OpenBrowser hands a URL to the user's default browser.
//
// The same host commands as OpenFinder: each takes a URL as readily as a path,
// and on Windows `explorer <url>` is what opens the default browser. Failure is
// not fatal to a caller that also prints the address - a headless machine, a
// container, or an SSH session has no browser to open, which is a normal way to
// run a local server rather than an error in it.
func OpenBrowser(url string) error {
	if err := openWithHostHandler(url); err != nil {
		return Errorf("failed to open browser: %w", err)
	}
	return nil
}

// openWithHostHandler asks the desktop environment to open a path or URL with
// whatever it considers the right application.
//
// Start rather than Run: the handler outlives this process, and waiting for it
// would block until the user closes the window it opened.
func openWithHostHandler(target string) error {
	var cmd string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "explorer"
	default: // linux and others
		cmd = "xdg-open"
	}

	return exec.Command(cmd, target).Start()
}
