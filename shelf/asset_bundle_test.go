package shelf

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestPCloudAssetBundleUsesTwoRequests(t *testing.T) {
	for _, n := range []int{1, 12} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			root := t.TempDir()
			local := newTestShelf(t, &ShelfConf{LibRoot: root, LockMode: "none", BookCacheWriterID: testWriterID})
			book, err := local.NewBook(FolderPath{}, "Illustrated")
			if err != nil {
				t.Fatal(err)
			}
			source, err := book.NewSource(strings.NewReader("text"))
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, n)
			for i := range names {
				names[i] = fmt.Sprintf("image-%02d.png", i)
				if err := source.WriteAsset(names[i], []byte(names[i])); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := local.ExportBookCache(); err != nil {
				t.Fatal(err)
			}
			s, srv := openPCloudShelfOffline(t, root, ShelfConf{}, t.TempDir())
			remoteBook, err := s.GetBook(book.ID())
			if err != nil {
				t.Fatal(err)
			}
			remoteSource, err := remoteBook.GetSource(source.ID())
			if err != nil {
				t.Fatal(err)
			}
			srv.Take()
			requested := append(slices.Clone(names), "missing.png", names[0])
			checkAssetBundle(t, remoteSource, requested, append(slices.Clone(names), names[0]))
			if got := srv.Take(); !slices.Equal(got, []string{"getziplink", "download"}) {
				t.Fatalf("requests = %v", got)
			}
			checkAssetBundle(t, remoteSource, []string{"missing.png"}, nil)
			checkAssetBundle(t, remoteSource, nil, nil)
			if got := srv.Take(); len(got) != 0 {
				t.Fatalf("empty bundle requests = %v", got)
			}
			if _, err := s.DownloadBook(book.ID()); err != nil {
				t.Fatal(err)
			}
			srv.SetOffline(true)
			srv.Take()
			checkAssetBundle(t, remoteSource, requested, append(slices.Clone(names), names[0]))
			if got := srv.Take(); len(got) != 0 {
				t.Fatalf("offline bundle requests = %v", got)
			}
		})
	}
}

func checkAssetBundle(t *testing.T, source *Source, requested, want []string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := source.WriteAssetsToZip(zw, requested); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range zr.File {
		got = append(got, entry.Name)
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != entry.Name || entry.Method != zip.Store {
			t.Fatalf("unexpected entry %q: %q, method %d", entry.Name, data, entry.Method)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}
