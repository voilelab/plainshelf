package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	// This is a one-off migration tool to move existing snapshots to sources.
	// 1. Folder snapshots -> sources
	// 2. Update book.json: current_snapshot -> current_source
	// 3. Update CURRENT_VERSION_LOCATION.txt if exists

	if len(os.Args) < 2 {
		println("Usage: migrate_snapshot_to_source <shelf_path>")
		os.Exit(1)
	}

	shelfPath := os.Args[1]
	log.Println("Starting migration of snapshots to sources in shelf:", shelfPath)

	booksDir := path.Join(shelfPath, "books")
	// recursively walk through books directory
	err := filepath.Walk(booksDir, func(bookPath string, info os.FileInfo, err error) error {
		// Need to check since path may not exist due to modified.
		if os.Stat(bookPath); os.IsNotExist(err) {
			log.Println("Book path does not exist, skipping:", bookPath)
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".novl") {
			return nil
		}

		log.Println("Processing book folder:", bookPath)

		snapshotsDir := path.Join(bookPath, "snapshots")
		sourcesDir := path.Join(bookPath, "sources")

		if _, err := os.Stat(snapshotsDir); os.IsNotExist(err) {
			log.Println("No snapshots directory found, skipping:", snapshotsDir)
			return nil
		}

		err = os.Rename(snapshotsDir, sourcesDir)
		if err != nil {
			return err
		}
		log.Println("Renamed snapshots to sources for book:", bookPath)

		bookMetaPath := path.Join(bookPath, "book.json")
		bookMetaBytes, err := os.ReadFile(bookMetaPath)
		if err != nil {
			return err
		}

		bookMetaStr := string(bookMetaBytes)
		bookMetaStr = strings.ReplaceAll(bookMetaStr, "current_snapshot", "current_source")

		err = os.WriteFile(bookMetaPath, []byte(bookMetaStr), 0644)
		if err != nil {
			return err
		}
		log.Println("Updated book.json for book:", bookPath)

		currentVersionLocationPath := path.Join(bookPath, "CURRENT_VERSION_LOCATION.txt")
		if _, err := os.Stat(currentVersionLocationPath); err == nil {
			cvBytes, err := os.ReadFile(currentVersionLocationPath)
			if err != nil {
				return err
			}

			cvStr := string(cvBytes)
			cvStr = strings.ReplaceAll(cvStr, "snapshots", "sources")

			err = os.WriteFile(currentVersionLocationPath, []byte(cvStr), 0644)
			if err != nil {
				return err
			}
			log.Println("Updated CURRENT_VERSION_LOCATION.txt for book:", bookPath)
		} else if !os.IsNotExist(err) {
			return err
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking through books directory: %v", err)
	}

	log.Println("Migration completed successfully.")
}
