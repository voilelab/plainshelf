package sources_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

// The handler must actually use the batch capability through AddVirtualShelf,
// while preserving the existing local endpoint's names and bytes.
func TestAPIPCloudSourceAssetsBundle(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Cloud Art", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	names := make([]string, 20)
	for i := range names {
		names[i] = fmt.Sprintf("chart %%20 %02d.png", i)
		env.WriteSourceAsset(t, created.Meta.ID, sourceID, names[i], []byte(names[i]))
	}
	names = append(names, "missing.png")
	localURL := apitest.AssetsBundleURL(created.Meta.ID, sourceID, names...)
	want := readZipBundle(t, env.Get(localURL).Body.Bytes())
	apitest.AssertStatus(t, env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)
	srv := pcloudtest.New(t, env.LibRoot, "/Shelf")
	client, err := pcloud.NewClient(srv.Host(), pcloudtest.Token, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	conf := shelf.ShelfConfWithID{ID: apitest.VirtualShelfID, ShelfConf: shelf.ShelfConf{ScanInterval: "1h"}}
	if err := env.App.AddVirtualShelf(conf, pcloud.NewFS(client, "/Shelf"), "test://pcloud-assets"); err != nil {
		t.Fatal(err)
	}
	s, ok := env.App.ShelfManager().GetShelf(apitest.VirtualShelfID)
	if !ok {
		t.Fatal("virtual shelf missing")
	}
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	srv.Take()
	rec := env.Get(strings.Replace(localURL, apitest.DefaultShelfID, apitest.VirtualShelfID, 1))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, "application/zip")
	got := readZipBundle(t, rec.Body.Bytes())
	if !slices.Equal(bundleEntryNames(got), bundleEntryNames(want)) {
		t.Fatalf("bundle names = %v, want %v", bundleEntryNames(got), bundleEntryNames(want))
	}
	for name, data := range want {
		if string(got[name]) != string(data) {
			t.Fatalf("asset %q = %q, want %q", name, got[name], data)
		}
	}
	if requests := srv.Take(); !slices.Contains(requests, "getziplink") {
		t.Fatalf("handler did not use batch download: %v", requests)
	}
}
