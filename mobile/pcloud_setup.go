package mobile

import (
	"context"
	"encoding/json/v2"
	"errors"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type pcloudShelfCheck struct {
	IsShelf bool `json:"is_shelf"`
	Books   int  `json:"books"`
}

// CheckPCloudShelf reports, as JSON {"is_shelf", "books"}, whether the folder
// at path is a shelf and how many books it holds. Books are counted, not read.
func CheckPCloudShelf(host, token, folder string) (out string, err error) {
	defer recoverInto(&err)
	client, err := pcloud.NewClient(host, token, pcloudHTTPClient)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	var check pcloudShelfCheck
	books, err := shelf.CountBookPackages(pcloud.NewFS(client, folder))
	switch {
	case errors.Is(err, shelf.ErrNotAShelf):
	case err != nil:
		return "", util.Errorf("%w", err)
	default:
		check = pcloudShelfCheck{IsShelf: true, Books: books}
	}
	return marshalJSON(check)
}

type pcloudSubfolder struct {
	Name     string `json:"name"`
	FolderID int64  `json:"folderid"`
}

type pcloudFolderListing struct {
	Folders []pcloudSubfolder `json:"folders"`
	IsShelf bool              `json:"is_shelf"`
}

// ListPCloudFolder lists one pCloud folder for the shelf-root picker, as JSON
// {"folders": [{"name", "folderid"}], "is_shelf"}. Book packages are left
// out: none can be a shelf root. 0 is the account root.
func ListPCloudFolder(host, token string, folderID int64) (out string, err error) {
	defer recoverInto(&err)
	client, err := pcloud.NewClient(host, token, pcloudHTTPClient)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	folders, err := client.ListFolders(context.Background(), folderID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	names := make([]string, len(folders))
	listing := pcloudFolderListing{Folders: []pcloudSubfolder{}}
	for i, f := range folders {
		names[i] = f.Name
		if !shelf.IsBookPackage(f.Name) {
			listing.Folders = append(listing.Folders, pcloudSubfolder{Name: f.Name, FolderID: f.ID})
		}
	}
	listing.IsShelf = shelf.IsShelfRoot(names)
	return marshalJSON(listing)
}

func marshalJSON(v any) (string, error) {
	bs, err := json.Marshal(v, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}
