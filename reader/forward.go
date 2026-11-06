package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const singleInstanceID = "com.voilelab.plainshelf-reader"

var errNotBookPackage = util.NewError("not a book package")

func (a *ReaderApp) OpenForwardedLaunch(argv []string, workingDir string) error {
	if len(argv) > 0 {
		argv = argv[1:]
	}
	launch, err := parseLaunchArgs(argv)
	if err != nil {
		return util.Errorf("forwarded launch arguments: %w", err)
	}
	defer a.native.focus()
	if launch.bookPath == "" {
		return nil
	}
	dir := launch.bookPath
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workingDir, dir)
	}
	return a.openForwardedBook(dir, launch.shelfID, launch.section)
}

func (a *ReaderApp) OpenFinderFile(path string) error {
	defer a.native.focus()
	return a.openForwardedBook(path, "", noLaunchSection)
}

func (a *ReaderApp) openForwardedBook(dir, shelfID string, section int) error {
	err := checkBookPackageDir(dir)
	if err == nil {
		_, err = a.openBook(dir, shelfID, section)
	}
	if err != nil {
		a.showNotABook()
		return util.Errorf("%w", err)
	}
	a.native.reloadApp()
	return nil
}

func checkBookPackageDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return util.Errorf("%q is not an absolute path: %w", dir, errNotBookPackage)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return util.Errorf("%w: %w", errNotBookPackage, err)
	}
	if !info.IsDir() {
		return util.Errorf("%q is not a folder: %w", dir, errNotBookPackage)
	}
	meta, err := os.Stat(filepath.Join(dir, bookpkg.BookMetaFile))
	if err != nil || !meta.Mode().IsRegular() {
		return util.Errorf("%q has no %s: %w", dir, bookpkg.BookMetaFile, errNotBookPackage)
	}
	return nil
}

func logForwardError(err error) {
	if err != nil {
		log.Println("failed to open a forwarded book package:", err)
	}
}
