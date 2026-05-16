package fsutil

import (
	"io"
	"io/fs"
)

// FS is a common interface for file system operations.
// It abstracts over different file system implementations, such as local file system .
type FS interface {
	// Open opens a file for reading.
	// User should call Close() on the returned file when done.
	Open(name string) (fs.File, error)

	// ReadDir reads the contents of the specified directory and returns a list of directory entries.
	ReadDir(name string) ([]fs.DirEntry, error)

	// Stat returns the FileInfo structure describing the specified file or directory.
	Stat(name string) (fs.FileInfo, error)

	// OpenWriter opens a file for writing.
	// If the file does not exist, it will be created.
	// If it already exists, it will be truncated.
	// User should call Close() on the returned WriteCloser when done.
	OpenWriter(name string) (io.WriteCloser, error)

	// Mkdir creates a new directory with the specified name and permissions.
	Mkdir(name string) error

	// MkdirAll creates a directory and all necessary parents.
	MkdirAll(path string) error

	// Rename renames (moves) oldPath to newPath.
	// If newPath already exists and is not a directory, Rename replaces it.
	// OS-specific restrictions may apply when oldPath and newPath are in different directories.
	Rename(oldPath, newPath string) error

	// Remove deletes the specified file or directory.
	Remove(name string) error

	// RemoveAll removes the specified file or directory and any children it contains.
	RemoveAll(name string) error
}
