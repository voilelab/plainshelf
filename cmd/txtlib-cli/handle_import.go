package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/txtlib"
)

func handleImport(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: sourcePath and bookID are required")
		fmt.Println("Usage: txtlib-cli import <sourcePath> <bookID> [path]")
		os.Exit(1)
	}

	sourcePath := args[0]
	bookID := args[1]

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

	// Convert paths to absolute
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		fmt.Printf("Error converting source path to absolute: %v\n", err)
		os.Exit(1)
	}

	absLibPath, err := filepath.Abs(libPath)
	if err != nil {
		fmt.Printf("Error converting library path to absolute: %v\n", err)
		os.Exit(1)
	}

	// Check if source file exists
	if _, err := os.Stat(absSourcePath); os.IsNotExist(err) {
		fmt.Printf("Error: Source file does not exist: %s\n", absSourcePath)
		os.Exit(1)
	}

	// Open the txtlib library
	lib, err := txtlib.OpenLocalLib(absLibPath)
	if err != nil {
		fmt.Printf("Error opening library: %v\n", err)
		os.Exit(1)
	}
	defer lib.Close()

	// Check if book already exists, if not create it
	book, err := lib.GetBook(bookID)
	if err != nil {
		// Book doesn't exist, create it with filename as title
		filename := filepath.Base(absSourcePath)
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

		fmt.Printf("Book '%s' does not exist, creating it with title '%s'...\n", bookID, nameWithoutExt)
		book, err = lib.NewBook([]string{"Uncategorized"}, nameWithoutExt)
		if err != nil {
			fmt.Printf("Error creating book: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Book created successfully!\n")
	}

	// Open source file
	sourceFile, err := os.Open(absSourcePath)
	if err != nil {
		fmt.Printf("Error opening source file: %v\n", err)
		os.Exit(1)
	}
	defer sourceFile.Close()

	// Determine source type from file extension
	ext := strings.ToLower(filepath.Ext(absSourcePath))
	var sourceType string
	switch ext {
	case ".txt":
		sourceType = "text"
	case ".md", ".markdown":
		sourceType = "markdown"
	default:
		sourceType = "file"
	}

	// Use filename as source label and full path as URI
	sourceLabel := filepath.Base(absSourcePath)
	sourceURI := "file://" + absSourcePath

	fmt.Printf("Importing content from '%s' into book '%s'...\n", absSourcePath, book.ID())
	fmt.Printf("Source type: %s\n", sourceType)

	// Create the snapshot
	snapshot, err := book.NewSnapshot(sourceFile, sourceType, sourceLabel, sourceURI)
	if err != nil {
		fmt.Printf("Error creating snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Import completed successfully!\n")
	fmt.Printf("Book ID:    %s\n", book.ID())
	fmt.Printf("Book Title: %s\n", book.Title())
	fmt.Printf("Snapshot:   %s\n", snapshot.ID())
	fmt.Printf("Source:     %s\n", sourceLabel)

	if book.CurrentSnapshot() == "" {
		// Set the imported snapshot as the current snapshot if there isn't one already
		err = book.SetCurrentSnapshot(snapshot.ID())
		if err != nil {
			fmt.Printf("Error setting current snapshot: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Set imported snapshot as current snapshot.\n")
	}
}
