package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/txtlib"
)

func handleInit(args []string) {
	var libPath string
	if len(args) > 0 {
		libPath = args[0]
	} else {
		var err error
		libPath, err = os.Getwd()
		if err != nil {
			fmt.Printf("Error getting current directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(libPath)
	if err != nil {
		fmt.Printf("Error converting path to absolute: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initializing library at: %s\n", absPath)

	lib, err := txtlib.OpenLocalLib(absPath)
	if err != nil {
		fmt.Printf("Error initializing library: %v\n", err)
		os.Exit(1)
	}
	defer lib.Close()

	fmt.Println("Library initialized successfully!")
}
