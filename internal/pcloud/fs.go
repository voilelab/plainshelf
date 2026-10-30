package pcloud

import (
	"context"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const DefaultListingMaxAge = 10 * time.Second

type FS struct {
	client *Client
	root   string

	ListingMaxAge time.Duration

	mu   sync.Mutex
	tree *node
	now  func() time.Time
}

var _ fsutil.ReadFS = (*FS)(nil)
var _ fsutil.ZipOpener = (*FS)(nil)

func NewFS(client *Client, root string) *FS {
	return &FS{client: client, root: root, ListingMaxAge: DefaultListingMaxAge, now: time.Now}
}

type node struct {
	name     string
	isDir    bool
	id       int64
	size     int64
	modified time.Time

	children map[string]*node
	listedAt time.Time
}

func (n *node) listed() bool {
	return n.children != nil
}

func newNode(it *item, listedAt time.Time, recursive bool) *node {
	n := &node{name: it.Name, isDir: it.IsFolder, size: it.Size}
	if t, err := time.Parse(time.RFC1123Z, it.Modified); err == nil {
		n.modified = t
	}
	switch {
	case it.IsFolder && it.FolderID != nil:
		n.id = *it.FolderID
	case !it.IsFolder && it.FileID != nil:
		n.id = *it.FileID
	}
	if it.IsFolder && (recursive || it.Contents != nil) {
		n.setChildren(it.Contents, listedAt, recursive, nil)
	}
	return n
}

func (n *node) setChildren(items []item, listedAt time.Time, recursive bool, previous map[string]*node) {
	n.children = make(map[string]*node, len(items))
	n.listedAt = listedAt
	for i := range items {
		child := newNode(&items[i], listedAt, recursive)
		if old, ok := previous[child.name]; ok && old.isDir && child.isDir && old.id == child.id && !child.listed() {
			child.children, child.listedAt = old.children, old.listedAt
		}
		n.children[child.name] = child
	}
}

func (f *FS) stale(n *node) bool {
	return !n.listed() || f.now().Sub(n.listedAt) > f.ListingMaxAge
}

func (f *FS) loadLocked(ctx context.Context) error {
	if f.tree != nil {
		return nil
	}
	id, err := f.client.resolveFolderID(ctx, f.root)
	if err != nil {
		return err
	}
	listing, err := f.client.listFolderRecursive(ctx, id)
	if err != nil {
		return err
	}
	f.tree = newNode(listing, f.now(), true)
	f.tree.name = "."
	return nil
}

func (f *FS) relistLocked(ctx context.Context, n *node) error {
	listing, err := f.client.listFolder(ctx, n.id, false)
	if err != nil {
		return err
	}
	n.setChildren(listing.Contents, f.now(), false, n.children)
	return nil
}

func (f *FS) lookup(op, name string) (*node, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	ctx := context.Background()

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.loadLocked(ctx); err != nil {
		return nil, &fs.PathError{Op: op, Path: name, Err: err}
	}
	n := f.tree
	if name == "." {
		return n, nil
	}
	for segment := range strings.SplitSeq(name, "/") {
		if !n.isDir {
			return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}
		if !n.listed() {
			if err := f.relistLocked(ctx, n); err != nil {
				return nil, &fs.PathError{Op: op, Path: name, Err: err}
			}
		}
		child, ok := n.children[segment]
		if !ok && f.stale(n) {
			if err := f.relistLocked(ctx, n); err != nil {
				return nil, &fs.PathError{Op: op, Path: name, Err: err}
			}
			child, ok = n.children[segment]
		}
		if !ok {
			return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}
		n = child
	}
	return n, nil
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	n, err := f.lookup("stat", name)
	if err != nil {
		return nil, err
	}
	return fileInfo{n: n}, nil
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	n, err := f.lookup("readdir", name)
	if err != nil {
		return nil, err
	}
	if !n.isDir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: util.NewError("not a directory")}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stale(n) {
		if err := f.relistLocked(context.Background(), n); err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
		}
	}
	return dirEntries(n), nil
}

func dirEntries(n *node) []fs.DirEntry {
	entries := make([]fs.DirEntry, 0, len(n.children))
	for _, name := range slices.Sorted(maps.Keys(n.children)) {
		entries = append(entries, fs.FileInfoToDirEntry(fileInfo{n: n.children[name]}))
	}
	return entries
}

func (f *FS) Open(name string) (fs.File, error) {
	n, err := f.lookup("open", name)
	if err != nil {
		return nil, err
	}
	if n.isDir {
		f.mu.Lock()
		entries := dirEntries(n)
		f.mu.Unlock()
		return &dirFile{info: fileInfo{n: n}, entries: entries}, nil
	}

	body, err := f.client.download("download", func(ctx context.Context) (string, error) {
		return f.client.fileLink(ctx, n.id)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &file{ReadCloser: body, info: fileInfo{n: n}}, nil
}

func (f *FS) OpenZip(names ...string) (io.ReadCloser, error) {
	if len(names) == 0 {
		return nil, util.NewError("no files to zip")
	}
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		n, err := f.lookup("open", name)
		if err != nil {
			return nil, err
		}
		if n.isDir {
			return nil, &fs.PathError{Op: "open", Path: name, Err: util.NewError("is a directory")}
		}
		ids = append(ids, n.id)
	}
	return f.client.download("zip download", func(ctx context.Context) (string, error) {
		return f.client.zipLink(ctx, ids)
	})
}

type fileInfo struct{ n *node }

func (i fileInfo) Name() string       { return path.Base(i.n.name) }
func (i fileInfo) Size() int64        { return i.n.size }
func (i fileInfo) ModTime() time.Time { return i.n.modified }
func (i fileInfo) IsDir() bool        { return i.n.isDir }
func (i fileInfo) Sys() any           { return nil }
func (i fileInfo) Mode() fs.FileMode {
	if i.n.isDir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

type file struct {
	io.ReadCloser
	info fileInfo
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }

type dirFile struct {
	info    fileInfo
	entries []fs.DirEntry
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dirFile) Close() error               { return nil }
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.Name(), Err: util.NewError("is a directory")}
}

func (d *dirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if count <= 0 {
		out := d.entries
		d.entries = nil
		return out, nil
	}
	if len(d.entries) == 0 {
		return nil, io.EOF
	}
	count = min(count, len(d.entries))
	out := d.entries[:count]
	d.entries = d.entries[count:]
	return out, nil
}
