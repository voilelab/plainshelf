package main

import (
	"flag"

	txtlibsrv "github.com/voilelab/plainshelf/txtlib-srv"
)

func main() {
	var confPath string
	flag.StringVar(&confPath, "conf", "", "path to config file")
	flag.Parse()

	if confPath == "" {
		flag.Usage()
		return
	}

	err := txtlibsrv.MainService(confPath)
	if err != nil {
		panic(err)
	}
}
