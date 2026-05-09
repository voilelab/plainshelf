package util

import (
	"os/exec"
	"runtime"
)

func OpenFinder(path string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{path}
	case "windows":
		cmd = "explorer"
		args = []string{path}
	default: // linux and others
		cmd = "xdg-open"
		args = []string{path}
	}

	err := exec.Command(cmd, args...).Start()
	if err != nil {
		return Errorf("failed to open file explorer: %w", err)
	}
	return nil
}
