package sources_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type comicOverlapResponse struct {
	Total    int `json:"total"`
	Hashed   int `json:"hashed"`
	Missing  int `json:"missing"`
	Overlaps []struct {
		A      string `json:"a"`
		B      string `json:"b"`
		PagesA int    `json:"pages_a"`
		PagesB int    `json:"pages_b"`
		Shared int    `json:"shared"`
	} `json:"overlaps"`
}

func pagePNG(t *testing.T, n int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 1, 1))
	img.SetGray(0, 0, color.Gray{Y: uint8(n)})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func addComic(t *testing.T, env *apitest.Env, title string, pages map[string]int) string {
	t.Helper()
	book := apitest.ImportTextBook(t, env, title, "", title+".txt", "placeholder "+title)
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	slices.Sort(names)
	sourceID := createImagesSource(t, env, book.Meta.ID, strings.Join(names, "\n")+"\n")
	for name, n := range pages {
		env.WriteSourceAsset(t, book.Meta.ID, sourceID, name, pagePNG(t, n))
	}
	return book.Meta.ID
}

func numberedPages(prefix string, from, to int) map[string]int {
	pages := map[string]int{}
	for n := from; n <= to; n++ {
		pages[prefix+strconv.Itoa(n)+".png"] = n
	}
	return pages
}

func backdateComicPages(t *testing.T, env *apitest.Env, bookID string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	root := filepath.Dir(env.BookMetaPath(t, bookID))
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(filepath.Dir(p)) != shelf.SourceAssetsFolder {
			return err
		}
		return os.Chtimes(p, old, old)
	})
	if err != nil {
		t.Fatalf("backdating pages: %v", err)
	}
}

func runHashComicPages(t *testing.T, env *apitest.Env) task.HashComicPagesResult {
	t.Helper()
	accepted := apitest.SubmitTaskChain(t, env, apitest.ComicPageHashesURL(), nil, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Name != task.HashComicPagesTaskName {
		t.Fatalf("chain = %+v, want a completed %s chain", chain, task.HashComicPagesTaskName)
	}
	return apitest.TaskResult[task.HashComicPagesResult](t, chain)
}

func groupOf(groups [][]string, bookID string) []string {
	for _, group := range groups {
		if slices.Contains(group, bookID) {
			return group
		}
	}
	return nil
}

func TestAPIComicSameCBZImportedTwice(t *testing.T) {
	env := apitest.New(t)
	archive := string(testutil.BuildCBZ(t, map[string][]byte{"1.png": pagePNG(t, 1), "2.png": pagePNG(t, 2)}))
	first := apitest.ImportFileBook(t, env, "first.cbz", "application/zip", archive)
	second := apitest.ImportFileBook(t, env, "second.cbz", "application/zip", archive)

	before := apitest.GetJSON[comicOverlapResponse](t, env, apitest.ComicOverlapsURL())
	if before.Total != 2 || before.Hashed != 0 || before.Missing != 2 {
		t.Fatalf("coverage before hashing = %+v, want two comics missing", before)
	}

	runHashComicPages(t, env)

	groups := apitest.GetJSON[[][]string](t, env, apitest.ShelfURL("books", "duplicate"))
	if group := groupOf(groups, first.Meta.ID); !slices.Contains(group, second.Meta.ID) {
		t.Fatalf("duplicate groups = %v, want the CBZ imported twice in one group", groups)
	}
	after := apitest.GetJSON[comicOverlapResponse](t, env, apitest.ComicOverlapsURL())
	if after.Hashed != 2 || after.Missing != 0 || len(after.Overlaps) != 0 {
		t.Fatalf("overlaps = %+v, want both hashed and no partial overlap for an exact duplicate", after)
	}
}

func TestAPIComicDuplicateRelations(t *testing.T) {
	env := apitest.New(t)
	base := addComic(t, env, "Base", numberedPages("p", 1, 10))
	reordered := map[string]int{}
	for name, n := range numberedPages("q", 1, 10) {
		reordered[name] = 11 - n
	}
	shuffled := addComic(t, env, "Shuffled", reordered)
	extra := addComic(t, env, "Extra", numberedPages("p", 1, 11))
	otherImages := numberedPages("p", 1, 10)
	for name := range otherImages {
		otherImages[name] += 100
	}
	sameNames := addComic(t, env, "SameNames", otherImages)
	for _, id := range []string{base, shuffled, extra, sameNames} {
		backdateComicPages(t, env, id)
	}

	first := runHashComicPages(t, env)
	if first.Comics != 4 || first.Hashed != 4 || first.PagesRead != 41 || len(first.Failures) != 0 {
		t.Fatalf("first run = %+v, want every page of four comics read", first)
	}

	groups := apitest.GetJSON[[][]string](t, env, apitest.ShelfURL("books", "duplicate"))
	if group := groupOf(groups, base); len(group) != 2 || !slices.Contains(group, shuffled) {
		t.Errorf("group of the base comic = %v, want it with the reordered copy only", group)
	}
	if groupOf(groups, extra) != nil {
		t.Errorf("groups = %v, want the comic with an extra page left out of exact groups", groups)
	}
	if group := groupOf(groups, sameNames); group != nil {
		t.Errorf("group = %v, want same page names with other images kept apart", group)
	}

	report := apitest.GetJSON[comicOverlapResponse](t, env, apitest.ComicOverlapsURL())
	extraPairs := 0
	for _, overlap := range report.Overlaps {
		if overlap.A != extra && overlap.B != extra {
			t.Errorf("overlap %+v, want only pairs with the comic that has an extra page", overlap)
			continue
		}
		extraPairs++
		if overlap.Shared != 10 || overlap.PagesA+overlap.PagesB != 21 {
			t.Errorf("overlap %+v, want 10 shared pages between 10 and 11", overlap)
		}
	}
	if extraPairs != 1 {
		t.Errorf("overlaps = %+v, want the extra-page comic paired once with the base group", report.Overlaps)
	}

	second := runHashComicPages(t, env)
	if second.PagesRead != 0 || second.StatHits != 41 {
		t.Errorf("second run = %+v, want every page answered from the cache", second)
	}
}

func TestAPIComicMissingPageLeftOut(t *testing.T) {
	env := apitest.New(t)
	complete := addComic(t, env, "Complete", map[string]int{"p1.png": 1})
	book := apitest.ImportTextBook(t, env, "Gappy", "", "gappy.txt", "placeholder gappy")
	sourceID := createImagesSource(t, env, book.Meta.ID, "p1.png\np2.png\n")
	env.WriteSourceAsset(t, book.Meta.ID, sourceID, "p1.png", pagePNG(t, 1))

	accepted := apitest.SubmitTaskChain(t, env, apitest.ComicPageHashesURL(), nil, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	result := apitest.TaskResult[task.HashComicPagesResult](t, chain)
	if chain.Status != "partially_completed" || len(result.Failures) != 1 || result.Failures[0].Code != "missing_pages" {
		t.Fatalf("chain %q, result %+v; want the comic with a missing page reported as missing_pages", chain.Status, result)
	}

	groups := apitest.GetJSON[[][]string](t, env, apitest.ShelfURL("books", "duplicate"))
	if groupOf(groups, complete) != nil {
		t.Fatalf("groups = %v, want a comic with a missing page kept out of exact groups", groups)
	}
}
