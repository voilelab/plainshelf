package cbz

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestArchiveLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		size  uint64
		want  string
	}{
		{"entry count", 2001, 0, "2000 entry"},
		{"entry size", 1, maxEntryBytes + 1, "20 MiB"},
		{"total size", 103, maxEntryBytes, "2 GiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for i := 0; i < tc.count; i++ {
				_, err := zw.CreateRaw(&zip.FileHeader{Name: fmt.Sprintf("%d.txt", i), Method: zip.Store, UncompressedSize64: tc.size})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			_, err := Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNaturalOrder(t *testing.T) {
	for _, pair := range [][2]string{
		{"2.jpg", "10.jpg"}, {"ch2/10.jpg", "ch10/1.jpg"},
		{"page9.jpg", "page1000000000000000000000000.jpg"},
		{"page001.jpg", "page1.jpg"}, {"PAGE1.JPG", "page1.jpg"},
	} {
		if naturalCompare(pair[0], pair[1]) >= 0 || naturalCompare(pair[1], pair[0]) <= 0 {
			t.Fatalf("order %q", pair)
		}
	}
}

func TestReadEntryBounds(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("bomb.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(make([]byte, maxEntryBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readEntry(zr.File[0]); err == nil || !strings.Contains(err.Error(), "20 MiB") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectSymlink(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: ".hidden/link.jpg"}
	header.SetMode(os.ModeSymlink | 0777)
	if _, err := zw.CreateHeader(header); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("err = %v", err)
	}
}
