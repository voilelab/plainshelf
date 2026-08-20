package fsutil

import (
	"io"
	"io/fs"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrReadOnly is returned when a write is requested through a handle that only
// carries read access. See Writable.
var ErrReadOnly = util.NewError("filesystem is read-only")

// ReadFS is the read-only half of FS.
//
// It exists so that "this handle is only ever read from" can be a property of
// the type rather than a naming convention: a value stored as a ReadFS cannot
// be written through without narrowing it back with Writable, which makes the
// compiler enumerate every mutation path.
type ReadFS interface {
	// Open opens a file for reading.
	// User should call Close() on the returned file when done.
	Open(name string) (fs.File, error)

	// ReadDir reads the contents of the specified directory and returns a list of directory entries.
	ReadDir(name string) ([]fs.DirEntry, error)

	// Stat returns the FileInfo structure describing the specified file or directory.
	Stat(name string) (fs.FileInfo, error)
}

// FS is a common interface for file system operations.
// It abstracts over different file system implementations, such as local file systems.
type FS interface {
	ReadFS

	// OpenWriter opens a file for writing.
	// If the file does not exist, it will be created.
	// If it already exists, it will be truncated.
	// User should call Close() on the returned WriteCloser when done.
	OpenWriter(name string) (io.WriteCloser, error)

	// WriteFile writes data to the specified file, creating it if it does not exist and truncating it if it does.
	WriteFile(name string, data []byte) error

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

// Writable narrows a read handle back to a writable one, reporting ErrReadOnly
// when the underlying implementation offers reads only.
//
// This is the single door between the two interfaces: a caller that holds a
// ReadFS asks for write access here, once, and gets an error it can return
// instead of a panic when the shelf behind it cannot be written.
func Writable(rfs ReadFS) (FS, error) {
	fsys, ok := rfs.(FS)
	if !ok {
		return nil, util.Errorf("%w", ErrReadOnly)
	}
	return fsys, nil
}
