package readerapi_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

func testAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<html><head><title>PlainShelf</title></head><body><div id=\"app\"></div></body></html>"),
		},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
}

func serveSPA(t *testing.T, boot readerapi.BootConfig, path string) *httptest.ResponseRecorder {
	t.Helper()

	handler := readerapi.NewSPAHandler(testAssets(), func() readerapi.BootConfig { return boot })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestSPAInjectsTheOpenBook(t *testing.T) {
	boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID}

	response := serveSPA(t, boot, "/")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	body := response.Body.String()
	if !strings.Contains(body, "window.__PLAINSHELF_READER__") {
		t.Error("expected the boot config to be injected")
	}
	if !strings.Contains(body, `"book_id":"`+testBookID+`"`) {
		t.Errorf("expected the open book in the boot config, got:\n%s", body)
	}
	if !strings.Contains(body, `"shelf_id":"`+readerapi.ShelfID+`"`) {
		t.Error("expected the shelf id in the boot config")
	}

	if strings.Index(body, "window.__PLAINSHELF_READER__") > strings.Index(body, "</head>") {
		t.Error("expected the boot config before </head>")
	}
}

func TestSPAInjectsTheLaunchSection(t *testing.T) {
	for _, section := range []int{0, 5} {
		boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID, Section: &section}

		body := serveSPA(t, boot, "/").Body.String()
		if want := `"section":` + strconv.Itoa(section); !strings.Contains(body, want) {
			t.Errorf("expected %q in the boot config, got:\n%s", want, body)
		}
	}
}

func TestSPAOmitsAnAbsentSection(t *testing.T) {
	boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID}

	if body := serveSPA(t, boot, "/").Body.String(); strings.Contains(body, `"section"`) {
		t.Errorf("expected no section in the boot config, got:\n%s", body)
	}
}

func TestSPAServesInAppRoutes(t *testing.T) {
	boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID}

	response := serveSPA(t, boot, "/reader/"+testBookID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `id="app"`) {
		t.Error("expected an in-app route to be answered with index.html")
	}
}

func TestSPAServesAssetsUntouched(t *testing.T) {
	response := serveSPA(t, readerapi.BootConfig{}, "/assets/app.js")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "console.log('app')" {
		t.Errorf("asset body = %q, want it unchanged", body)
	}
}

func TestSPAMarksTheReaderRuntimeWithNoBook(t *testing.T) {
	response := serveSPA(t, readerapi.BootConfig{ShelfID: readerapi.ShelfID}, "/")

	body := response.Body.String()
	if !strings.Contains(body, "window.__PLAINSHELF_READER__") {
		t.Error("expected the reader marker even with no book open")
	}
	if !strings.Contains(body, `"book_id":""`) {
		t.Errorf("expected an empty book id, got:\n%s", body)
	}
}

func TestSPAEscapesScriptCloseInTheBootConfig(t *testing.T) {
	boot := readerapi.BootConfig{
		ShelfID: readerapi.ShelfID,
		BookID:  `b</script><script>alert(1)</script>`,
	}

	body := serveSPA(t, boot, "/").Body.String()

	if strings.Contains(body, "</script><script>") {
		t.Errorf("the book ID closed the boot script:\n%s", body)
	}
	if !strings.Contains(body, `b\u003c/script`) {
		t.Errorf("expected the book ID escaped, got:\n%s", body)
	}
}
