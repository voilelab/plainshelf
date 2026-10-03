// Package shelfzip moves a shelf in and out of the wasm demo as a zip whose
// layout is the shelf directory itself, so it unzips into a native lib_root.
// Only books/ and trash/ travel: app/ holds this installation's caches.
package shelfzip

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// shelfDirs are the parts of a lib_root a zip carries.
var shelfDirs = []string{"books", "trash"}

// maxImportBytes caps what an import may unpack; the demo holds it all in memory.
const maxImportBytes = 512 << 20

var (
	// ErrNotShelf rejects a zip with nothing under books/.
	ErrNotShelf = errors.New("not a PlainShelf shelf: the zip has no books/ directory")
	// ErrUnsafePath rejects an entry that would land outside the shelf.
	ErrUnsafePath = errors.New("zip entry escapes the shelf")
	// ErrTooLarge rejects a zip that unpacks past maxImportBytes.
	ErrTooLarge = errors.New("zip unpacks to more than 512 MiB")
)

// Export writes books/ and trash/ under libRoot to w.
func Export(w io.Writer, libRoot string) error {
	zw := zip.NewWriter(w)
	for _, dir := range shelfDirs {
		err := filepath.WalkDir(filepath.Join(libRoot, dir), func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && p == filepath.Join(libRoot, dir) {
				return fs.SkipDir
			}
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(libRoot, p)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(rel)
			if d.IsDir() {
				// Directory entries keep empty folders.
				_, err := zw.Create(name + "/")
				return err
			}
			return addFile(zw, p, name)
		})
		if err != nil {
			return util.Errorf("%w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func addFile(zw *zip.Writer, p, name string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	dst, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, f)
	return err
}

// Import replaces books/ and trash/ under libRoot with the zip's. Every entry
// is checked before anything is touched; the zip unpacks next to the shelf
// first, so a failure part-way leaves the existing books in place.
func Import(r io.ReaderAt, size int64, libRoot string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return util.Errorf("%w", err)
	}
	entries, err := plan(zr.File)
	if err != nil {
		return err
	}

	staging := libRoot + ".import"
	if err := os.RemoveAll(staging); err != nil {
		return util.Errorf("%w", err)
	}
	defer os.RemoveAll(staging)
	for _, dir := range shelfDirs {
		if err := os.MkdirAll(filepath.Join(staging, dir), 0o755); err != nil {
			return util.Errorf("%w", err)
		}
	}
	for _, e := range entries {
		if err := extract(e.file, filepath.Join(staging, filepath.FromSlash(e.name))); err != nil {
			return util.Errorf("%w", err)
		}
	}
	for _, dir := range shelfDirs {
		target := filepath.Join(libRoot, dir)
		if err := os.RemoveAll(target); err != nil {
			return util.Errorf("%w", err)
		}
		if err := os.Rename(filepath.Join(staging, dir), target); err != nil {
			return util.Errorf("%w", err)
		}
	}
	return nil
}

type entry struct {
	file *zip.File
	name string // slash path relative to lib_root, starting with books/ or trash/
}

// plan maps zip entries onto the shelf, or rejects the whole zip. A single
// wrapping folder, as zipping a lib_root from a file manager produces, is
// stripped.
func plan(files []*zip.File) ([]entry, error) {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = strings.TrimPrefix(f.Name, "./")
	}
	prefix := wrapper(names)

	var out []entry
	var total uint64
	hasBooks := false
	for i, f := range files {
		name := strings.TrimPrefix(names[i], prefix)
		clean := path.Clean(name)
		if name == "" || clean == "." {
			continue
		}
		if strings.HasPrefix(name, "/") || strings.Contains(name, `\`) || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, util.Errorf("%w: %q", ErrUnsafePath, f.Name)
		}
		top, _, _ := strings.Cut(clean, "/")
		if top != "books" && top != "trash" {
			continue // app/ and stray files stay behind
		}
		if top == "books" {
			hasBooks = true
		}
		if f.FileInfo().IsDir() {
			if clean == top {
				continue
			}
			out = append(out, entry{file: f, name: clean + "/"})
			continue
		}
		total += f.UncompressedSize64
		if total > maxImportBytes {
			return nil, ErrTooLarge
		}
		out = append(out, entry{file: f, name: clean})
	}
	if !hasBooks {
		return nil, ErrNotShelf
	}
	return out, nil
}

// wrapper returns "name/" when every entry sits under one folder that is not
// itself a shelf directory, and "" otherwise.
func wrapper(names []string) string {
	first := ""
	for _, n := range names {
		top, _, nested := strings.Cut(n, "/")
		if !nested || top == "books" || top == "trash" || top == "app" {
			return ""
		}
		if first == "" {
			first = top
		} else if top != first {
			return ""
		}
	}
	if first == "" {
		return ""
	}
	return first + "/"
}

func extract(f *zip.File, dst string) error {
	if f.FileInfo().IsDir() {
		return os.MkdirAll(dst, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	// archive/zip fails a read past the declared size, which plan already counted.
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
