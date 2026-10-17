package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/voilelab/plainshelf/server"
)

func main() {
	var confPath string
	var rotateDAVToken bool
	flag.StringVar(&confPath, "conf", "", "path to config file")
	flag.BoolVar(&rotateDAVToken, "rotate-dav-token", false, "replace the dav_export token, print the new one and exit")
	flag.Parse()

	if confPath == "" {
		flag.Usage()
		return
	}

	if rotateDAVToken {
		token, err := server.RotateDAVExportTokenForConf(confPath)
		if err != nil {
			log.Println("Error rotating dav_export token:", err)
			os.Exit(1)
		}
		fmt.Println(token)
		return
	}

	err := server.MainService(confPath)
	if err != nil {
		log.Println("Error starting plainshelf-srv:", err)
		os.Exit(1)
	}
}
