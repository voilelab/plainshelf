package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/txtlib"
)

func handleList(args []string) {
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

	lib, err := txtlib.OpenLocalLib(absPath)
	if err != nil {
		fmt.Printf("Error opening library: %v\n", err)
		os.Exit(1)
	}
	defer lib.Close()

	books, err := lib.ListBooks()
	if err != nil {
		fmt.Printf("Error listing books: %v\n", err)
		os.Exit(1)
	}

	if len(books) == 0 {
		fmt.Println("No books found in the library.")
		return
	}

	fmt.Printf("Found %d book(s):\n", len(books))
	fmt.Println(strings.Repeat("-", 50))
	for _, book := range books {
		fmt.Printf("ID:    %s\n", book.ID())
		fmt.Printf("Title: %s\n", book.Title())
		if book.CurrentSnapshot() != "" {
			fmt.Printf("Snapshot: %s\n", book.CurrentSnapshot())
		}
		fmt.Println(strings.Repeat("-", 50))
	}
}
