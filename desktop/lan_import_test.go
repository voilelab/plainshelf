package main

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/lanshare"
)

func TestLANImportRedactsKeychainErrors(t *testing.T) {
	url, token := davExport(t)
	secrets := newFakeSecrets()
	secrets.setErr = errors.New("keychain refused secret " + token)
	a := desktopAppWithSecrets(t, secrets)
	text := importText(t, url, token, []lanshare.Shelf{{ID: "one", Name: "One", URL: url}})
	results, err := a.ImportLANShareQR(text)
	if err != nil || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("import results: %+v, %v", results, err)
	}
	body, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), token) {
		t.Fatal("keychain error exposed the LAN share token")
	}
	if len(secrets.m) != 0 {
		t.Fatal("failed keychain write saved the secret")
	}
}

func importText(t *testing.T, root, token string, shelves []lanshare.Shelf) string {
	t.Helper()
	b, err := json.Marshal(lanshare.Payload{V: 1, URL: root, Token: token, Shelves: shelves})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLANImportValidatesAllBeforeSideEffects(t *testing.T) {
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)
	before, _ := os.ReadFile(a.shelvesConfigPath)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer srv.Close()
	text := importText(t, srv.URL, "secret", []lanshare.Shelf{{ID: "one", Name: "One", URL: srv.URL + "/one/"}, {ID: "bad", Name: "Bad", URL: "ftp://invalid/"}})
	if _, err := a.PreviewLANShareQR(text); err == nil {
		t.Fatal("preview accepted invalid payload")
	}
	if _, err := a.ImportLANShareQR(text); err == nil {
		t.Fatal("import accepted invalid payload")
	}
	after, _ := os.ReadFile(a.shelvesConfigPath)
	if string(before) != string(after) || requests != 0 || len(secrets.m) != 0 {
		t.Fatal("invalid payload had side effects")
	}
}

func TestLANImportPartialRetry(t *testing.T) {
	url, token := davExport(t)
	root := strings.TrimSuffix(url, "default_shelf/")
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)
	text := importText(t, root, token, []lanshare.Shelf{{ID: "missing", Name: "Missing", URL: root + "missing/"}, {ID: "default_shelf", Name: "Living room", URL: url}})
	preview, err := a.PreviewLANShareQR(text)
	if err != nil || len(preview) != 2 || len(secrets.m) != 0 {
		t.Fatal("preview should only validate")
	}
	results, err := a.ImportLANShareQR(text)
	if err != nil || len(results) != 2 || results[0].Status == "added" || results[1].Status != "added" {
		t.Fatalf("import results: %+v, %v", results, err)
	}
	conf, _ := loadDesktopShelves(a.shelvesConfigPath)
	if len(conf.VirtualShelves) != 1 || len(secrets.m) != 1 || secrets.m["living-room"] != token {
		t.Fatal("failed entry saved or token not in keychain")
	}
	raw, _ := os.ReadFile(a.shelvesConfigPath)
	resultJSON, _ := json.Marshal(results)
	if strings.Contains(string(raw), token) || strings.Contains(string(resultJSON), token) {
		t.Fatal("token leaked")
	}

	retry := importText(t, root, "new-token", []lanshare.Shelf{{ID: "default_shelf", Name: "New name", URL: url}})
	results, err = a.ImportLANShareQR(retry)
	if err != nil || results[0].Status != "exists" || secrets.m["living-room"] != token {
		t.Fatal("duplicate replaced entry or secret")
	}
}

func TestLANImportWrongTokenDoesNotSave(t *testing.T) {
	url, _ := davExport(t)
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)
	text := importText(t, url, "wrong-token", []lanshare.Shelf{{ID: "one", Name: "One", URL: url}})
	results, err := a.ImportLANShareQR(text)
	if err != nil || results[0].Status != "SOURCE_UNAUTHORIZED" {
		t.Fatalf("results: %+v, %v", results, err)
	}
	conf, _ := loadDesktopShelves(a.shelvesConfigPath)
	if len(conf.VirtualShelves) != 0 || len(secrets.m) != 0 {
		t.Fatal("failed check saved data")
	}
}
