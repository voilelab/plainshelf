package shelf

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/davfs"
	"golang.org/x/net/webdav"
)

// Batch reads cross both the read-only wrapper and the offline overlay. The
// wire archive must match individual reads, even when pCloud compresses it.
func TestSourceAssetBundleAcrossBackends(t *testing.T) {
	for _, count := range []int{1, 20} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			libRoot := t.TempDir()
			local := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
			book, err := local.NewBook(FolderPath{}, "Illustrated")
			if err != nil {
				t.Fatal(err)
			}
			source, err := book.NewSource(strings.NewReader("body"))
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("chart %%20 %02d.png", i)
				if err := source.WriteAsset(names[i], []byte("image "+names[i])); err != nil {
					t.Fatal(err)
				}
			}
			dirPath, err := source.AssetPath("directory.png")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(libRoot, dirPath), 0755); err != nil {
				t.Fatal(err)
			}
			requested := append(slices.Clone(names), "missing.png", "directory.png", names[0])
			want := sourceBundle(t, source, requested)
			if len(want) != count+1 {
				t.Fatalf("local bundle has %d entries, want %d", len(want), count+1)
			}

			for _, backend := range []string{"pcloud", "pcloud-overlay", "pcloud-downloaded", "webdav"} {
				t.Run(backend, func(t *testing.T) {
					var remote *Shelf
					var take func() []string
					if backend == "webdav" {
						srv := httptest.NewServer(&webdav.Handler{
							FileSystem: webdav.Dir(libRoot), LockSystem: webdav.NewMemLS(),
						})
						t.Cleanup(srv.Close)
						root, err := davfs.New(srv.URL, davfs.Options{})
						if err != nil {
							t.Fatal(err)
						}
						remote, err = NewShelfFromFS(&ShelfConf{}, root, t.TempDir())
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { remote.Close() })
						if err := remote.WaitReady(t.Context()); err != nil {
							t.Fatal(err)
						}
					} else {
						offlineDir := ""
						if backend != "pcloud" {
							offlineDir = t.TempDir()
						}
						s, srv := openPCloudShelfOffline(t, libRoot, ShelfConf{}, offlineDir)
						remote, take = s, srv.Take
						if backend == "pcloud-downloaded" {
							if _, err := remote.DownloadBook(book.ID()); err != nil {
								t.Fatal(err)
							}
							srv.SetOffline(true)
						}
					}
					b, err := remote.GetBook(book.ID())
					if err != nil {
						t.Fatal(err)
					}
					src, err := b.GetSource(source.ID())
					if err != nil {
						t.Fatal(err)
					}
					if take != nil {
						take() // metadata loading is separate from the asset transfer
					}
					bundleNames := requested
					missingNames := []string{"missing.png", "directory.png"}
					if backend == "webdav" {
						// WebDAV GET on a directory is refused by the exporter;
						// this is separate from an absent asset's 404 contract.
						bundleNames = slices.DeleteFunc(slices.Clone(requested), func(name string) bool { return name == "directory.png" })
						missingNames = []string{"missing.png"}
					}
					got := sourceBundle(t, src, bundleNames)
					if !slices.Equal(got, want) {
						t.Fatal("bundle differs from local Store archive")
					}
					if take != nil {
						wantRequests := []string{"getziplink", "download"}
						if backend == "pcloud-downloaded" {
							wantRequests = nil
						}
						if got := take(); !slices.Equal(got, wantRequests) {
							t.Fatalf("requests = %v, want %v", got, wantRequests)
						}
					}
					// All missing is a valid empty archive, without a batch call.
					if entries := sourceBundle(t, src, missingNames); len(entries) != 0 {
						t.Fatalf("missing assets bundle = %v", entries)
					}
					if take != nil && len(take()) != 0 {
						t.Fatal("empty bundle contacted pCloud")
					}
					var buf bytes.Buffer
					if err := src.WriteAssetsZip(zip.NewWriter(&buf), []string{names[0], "../secret.png"}); err == nil || buf.Len() != 0 {
						t.Fatalf("unsafe bundle wrote %d bytes, error = %v", buf.Len(), err)
					}
				})
			}
		})
	}
}

type bundledAsset struct {
	name string
	data string
}

func sourceBundle(t *testing.T, source *Source, names []string) []bundledAsset {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := source.WriteAssetsZip(zw, names); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var assets []bundledAsset
	for _, entry := range zr.File {
		if entry.Method != zip.Store || strings.ContainsAny(entry.Name, "/\\") {
			t.Fatalf("unexpected entry header: %+v", entry.FileHeader)
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		assets = append(assets, bundledAsset{entry.Name, string(data)})
		if err != nil || string(data) != "image "+entry.Name {
			t.Fatalf("asset %q = %q, %v", entry.Name, data, err)
		}
	}
	return assets
}
