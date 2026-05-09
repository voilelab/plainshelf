package main

import (
	"fmt"
	"os"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "init":
		handleInit(args)
	case "list":
		handleList(args)
	case "add":
		handleAdd(args)
	case "import":
		handleImport(args)
	case "version", "--version", "-v":
		fmt.Printf("txtlib-cli version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("txtlib-cli - Personal Text Library Management")
	fmt.Printf("Version: %s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  txtlib-cli init [path]          Initialize a new library (default: current directory)")
	fmt.Println("  txtlib-cli list [path]          List all books in the library (default: current directory)")
	fmt.Println("  txtlib-cli add <title> [path]   Add a new book to the library")
	fmt.Println("  txtlib-cli import <sourcePath> <bookID> [path]    Import a book from a source path into the library")
	fmt.Println("")
	fmt.Println("Global options:")
	fmt.Println("  -h, --help     Show this help message")
	fmt.Println("  -v, --version  Show version information")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  txtlib-cli init                    # Initialize library in current directory")
	fmt.Println("  txtlib-cli init ~/my-library       # Initialize library in ~/my-library")
	fmt.Println("  txtlib-cli list                    # List books in current directory library")
	fmt.Println("  txtlib-cli add \"My Book\"         # Add a book with title 'My Book' to current directory library")
	fmt.Println("  txtlib-cli import ~/source/book.txt # Import a book from source path")
}
