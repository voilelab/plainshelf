package txtlibfrontend

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

var WebFS fs.FS

func init() {
	var err error
	WebFS, err = fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
}
