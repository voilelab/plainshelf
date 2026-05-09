package main

import (
	"flag"
	"log"
	"os"

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
		log.Println("Error starting txtlib-srv:", err)
		os.Exit(1)
	}
}
