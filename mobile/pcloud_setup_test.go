package mobile

import (
	"encoding/json/v2"
	"testing"

	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
)

func TestCheckPCloudShelfCountsBooks(t *testing.T) {
	srv, _ := servePCloudShelf(t)

	out, err := CheckPCloudShelf(srv.Host(), pcloudtest.Token, pcloudFolder)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"is_shelf":true,"books":2}` {
		t.Errorf("shelf check = %s", out)
	}

	out, err = CheckPCloudShelf(srv.Host(), pcloudtest.Token, "/PlainShelf")
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"is_shelf":false,"books":0}` {
		t.Errorf("parent folder check = %s", out)
	}

	if _, err := CheckPCloudShelf(srv.Host(), pcloudtest.Token, "/Missing"); err == nil {
		t.Error("missing folder: no error")
	}
	if srv.TokenLeaked() {
		t.Error("token leaked into a URL")
	}
}

func listPCloudFolder(t *testing.T, srv *pcloudtest.Server, id int64) pcloudFolderListing {
	t.Helper()
	out, err := ListPCloudFolder(srv.Host(), pcloudtest.Token, id)
	if err != nil {
		t.Fatal(err)
	}
	var listing pcloudFolderListing
	if err := json.Unmarshal([]byte(out), &listing); err != nil {
		t.Fatal(err)
	}
	return listing
}

func folderID(t *testing.T, listing pcloudFolderListing, name string) int64 {
	t.Helper()
	for _, f := range listing.Folders {
		if f.Name == name {
			return f.FolderID
		}
	}
	t.Fatalf("no folder %q in %+v", name, listing.Folders)
	return 0
}

func TestListPCloudFolderEntries(t *testing.T) {
	srv, _ := servePCloudShelf(t)

	root := listPCloudFolder(t, srv, 0)
	if root.IsShelf {
		t.Error("account root reads as a shelf")
	}
	parent := listPCloudFolder(t, srv, folderID(t, root, "PlainShelf"))
	shelfRoot := listPCloudFolder(t, srv, folderID(t, parent, "Shelf"))
	if !shelfRoot.IsShelf {
		t.Errorf("shelf root not marked: %+v", shelfRoot)
	}
	books := listPCloudFolder(t, srv, folderID(t, shelfRoot, "books"))
	if books.IsShelf || len(books.Folders) != 0 {
		t.Errorf("books/ = %+v, want no folders: both are book packages", books)
	}
}
