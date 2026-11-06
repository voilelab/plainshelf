package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func darwinLaunch(t *testing.T, bookPath, shelfID string, section int) readerLaunch {
	t.Helper()
	launch, err := readerLaunchFor("darwin", bookPath, shelfID, section)
	if err != nil {
		t.Fatalf("readerLaunchFor: %v", err)
	}
	return launch
}

func TestReaderLaunchCommandDefault(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	launch := darwinLaunch(t, "/books/alpha.bookpkg", "shelf-real", -1)
	if launch.name != "open" || launch.detach {
		t.Fatalf("launcher = %q detach=%v, want open and waited on", launch.name, launch.detach)
	}
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-shelf", "shelf-real"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func TestReaderLaunchCommandEnvOverride(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "/tmp/build/PlainShelfReader.app")

	launch := darwinLaunch(t, "/books/beta.bookpkg", "shelf-real", -1)
	want := []string{"-n", "-a", "/tmp/build/PlainShelfReader.app", "--args", "-book", "/books/beta.bookpkg", "-shelf", "shelf-real"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func TestReaderLaunchCommandNoShelf(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	launch := darwinLaunch(t, "/books/alpha.bookpkg", "", -1)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func TestReaderLaunchCommandWithSection(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	launch := darwinLaunch(t, "/books/alpha.bookpkg", "shelf-real", 3)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-shelf", "shelf-real", "-section", "3"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func TestReaderLaunchCommandSectionZero(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")

	launch := darwinLaunch(t, "/books/alpha.bookpkg", "", 0)
	want := []string{"-n", "-a", "PlainShelfReader", "--args", "-book", "/books/alpha.bookpkg", "-section", "0"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func fakeReaderExecutable(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake reader is a shell script")
	}
	path := filepath.Join(dir, readerExecutableName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func stubDesktopExecutable(t *testing.T, path string) {
	t.Helper()
	old := desktopExecutable
	desktopExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { desktopExecutable = old })
}

func TestReaderLaunchLinuxPrefersSibling(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")
	appDir, pathDir := t.TempDir(), t.TempDir()
	sibling := fakeReaderExecutable(t, appDir)
	fakeReaderExecutable(t, pathDir)
	t.Setenv("PATH", pathDir)
	stubDesktopExecutable(t, filepath.Join(appDir, "plainshelf"))

	launch, err := readerLaunchFor("linux", "/books/alpha.bookpkg", "shelf-real", 2)
	if err != nil {
		t.Fatalf("readerLaunchFor: %v", err)
	}
	if launch.name != sibling || !launch.detach {
		t.Fatalf("launcher = %q detach=%v, want %q started without waiting", launch.name, launch.detach, sibling)
	}
	want := []string{"-book", "/books/alpha.bookpkg", "-shelf", "shelf-real", "-section", "2"}
	if !reflect.DeepEqual(launch.args, want) {
		t.Fatalf("args = %v, want %v", launch.args, want)
	}
}

func TestReaderLaunchLinuxFallsBackToPath(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")
	pathDir := t.TempDir()
	onPath := fakeReaderExecutable(t, pathDir)
	t.Setenv("PATH", pathDir)
	stubDesktopExecutable(t, filepath.Join(t.TempDir(), "plainshelf"))

	launch, err := readerLaunchFor("linux", "/books/alpha.bookpkg", "", -1)
	if err != nil {
		t.Fatalf("readerLaunchFor: %v", err)
	}
	if launch.name != onPath {
		t.Fatalf("launcher = %q, want the reader on PATH %q", launch.name, onPath)
	}
}

func TestReaderLaunchLinuxEnvOverride(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "/opt/reader/PlainShelfReader")
	t.Setenv("PATH", t.TempDir())

	launch, err := readerLaunchFor("linux", "/books/alpha.bookpkg", "", -1)
	if err != nil {
		t.Fatalf("readerLaunchFor: %v", err)
	}
	if launch.name != "/opt/reader/PlainShelfReader" {
		t.Fatalf("launcher = %q, want the override", launch.name)
	}
}

func TestReaderLaunchLinuxMissingReader(t *testing.T) {
	t.Setenv("PLAINSHELF_READER_APP", "")
	t.Setenv("PATH", t.TempDir())
	stubDesktopExecutable(t, filepath.Join(t.TempDir(), "plainshelf"))

	if _, err := readerLaunchFor("linux", "/books/alpha.bookpkg", "", -1); err == nil {
		t.Fatal("readerLaunchFor succeeded without any PlainShelfReader to start")
	}
}

func TestReaderLaunchStartsDetached(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake reader is a shell script")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "args")
	script := filepath.Join(dir, readerExecutableName)
	body := "#!/bin/sh\necho \"$@\" > " + marker + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	launch := readerLaunch{name: script, args: []string{"-book", "/books/a.bookpkg"}, detach: true}
	if err := launch.run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := os.ReadFile(marker)
		if err == nil && string(got) == "-book /books/a.bookpkg\n" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("reader never ran with the book arguments (read %q, err %v)", got, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
