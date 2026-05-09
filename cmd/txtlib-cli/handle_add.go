package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/txtlib"
)

func handleAdd(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: bookID and title are required")
		fmt.Println("Usage: txtlib-cli add <bookID> <title> [path]")
		os.Exit(1)
	}

	bookID := args[0]
	title := args[1]

	var libPath string
	if len(args) > 2 {
		libPath = args[2]
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

	// Check if book already exists
	existingBook, err := lib.GetBook(bookID)
	if err == nil && existingBook != nil {
		fmt.Printf("Error: Book with ID '%s' already exists (Title: %s)\n", bookID, existingBook.Title())
		os.Exit(1)
	}

	fmt.Printf("Adding book '%s' with title '%s'...\n", bookID, title)

	book, err := lib.NewBook([]string{"Uncategorized"}, title)
	if err != nil {
		fmt.Printf("Error creating book: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Book created successfully!\n")
	fmt.Printf("ID:    %s\n", book.ID())
	fmt.Printf("Title: %s\n", book.Title())
}
