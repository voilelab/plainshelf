package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/txtlib"
)

func handleAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: title is required")
		fmt.Println("Usage: txtlib-cli add <title> [path]")
		os.Exit(1)
	}

	title := args[0]

	var libPath string
	if len(args) > 1 {
		libPath = args[1]
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

	lib, err := txtlib.OpenLocalLib(absPath)
	if err != nil {
		fmt.Printf("Error opening library: %v\n", err)
		os.Exit(1)
	}
	defer lib.Close()

	book, err := lib.NewBook([]string{"Uncategorized"}, title)
	if err != nil {
		fmt.Printf("Error creating book: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Book created successfully!\n")
	fmt.Printf("ID:    %s\n", book.ID())
	fmt.Printf("Title: %s\n", book.Title())
}
