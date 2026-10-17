package main

import (
	"encoding/json/v2"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

type lanShareEnv struct {
	dataRoot     string
	shelvesConf  string
	serverApp    *server.App
	share        *lanShare
	shelfRootsBy map[string]string
}

// newLANShareEnv starts two shelves, "a" and "b", and binds the export to
// loopback on a free port.
func newLANShareEnv(t *testing.T) *lanShareEnv {
	t.Helper()
	oldPort, oldAddrs, oldListen, oldSave := lanShareDefaultPort, lanAddresses, lanListen, saveLANShare
	lanShareDefaultPort = 0
	lanAddresses = func() ([]string, error) { return []string{"127.0.0.1"}, nil }
	t.Cleanup(func() { lanShareDefaultPort, lanAddresses, lanListen, saveLANShare = oldPort, oldAddrs, oldListen, oldSave })

	dataRoot := t.TempDir()
	env := &lanShareEnv{
		dataRoot:     dataRoot,
		shelvesConf:  filepath.Join(dataRoot, "shelves.json"),
		shelfRootsBy: map[string]string{"a": t.TempDir(), "b": t.TempDir()},
	}
	conf := &desktopShelvesConfig{}
	var shelves []*shelf.ShelfConfWithID
	for _, id := range []string{"a", "b"} {
		conf.Shelves = append(conf.Shelves, desktopShelfEntry{ID: id, Name: "Shelf " + id, LibRoot: env.shelfRootsBy[id]})
		shelves = append(shelves, &shelf.ShelfConfWithID{ID: id, ShelfConf: shelf.ShelfConf{LibRoot: env.shelfRootsBy[id]}})
	}
	if err := saveDesktopShelves(env.shelvesConf, conf); err != nil {
		t.Fatal(err)
	}

	app, err := server.NewApp(&server.AppConf{
		Shelves:   shelves,
		StorePath: t.TempDir(),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	for _, data := range app.ShelfManager().GetAllShelves() {
		if err := data.WaitReady(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	env.serverApp = app
	env.share = env.reopen(t)
	return env
}

// reopen builds the share as the next app start would.
func (e *lanShareEnv) reopen(t *testing.T) *lanShare {
	t.Helper()
	s := newLANShare(e.serverApp, e.dataRoot, e.shelvesConf)
	t.Cleanup(s.stop)
	return s
}

func lanGet(t *testing.T, st LANShareStatus, path, token string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), "PROPFIND", st.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Depth", "0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PROPFIND %s: %v", path, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestLANShareOffByDefault(t *testing.T) {
	env := newLANShareEnv(t)
	env.share.startSaved()
	st, err := env.share.status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Running || st.URL != "" || st.Token != "" {
		t.Fatalf("fresh status = %+v, want off", st)
	}
	for _, s := range st.Shelves {
		if s.Shared {
			t.Errorf("shelf %s shared by default", s.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(env.dataRoot, lanShareTokenName)); !os.IsNotExist(err) {
		t.Errorf("token file exists before sharing was turned on: %v", err)
	}
}

func TestLANShareStartStop(t *testing.T) {
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.Host != "127.0.0.1" || !strings.HasPrefix(st.URL, "http://127.0.0.1:") || !strings.HasSuffix(st.URL, "/dav/") {
		t.Fatalf("status after enabling = %+v", st)
	}
	if st.Token == "" || !strings.HasPrefix(st.QRCode, "data:image/png;base64,") {
		t.Fatalf("missing token or QR code: %+v", st)
	}

	if code := lanGet(t, st, "a/", st.Token); code != http.StatusMultiStatus {
		t.Errorf("PROPFIND shared shelf = %d, want 207", code)
	}
	if code := lanGet(t, st, "a/", ""); code != http.StatusUnauthorized {
		t.Errorf("PROPFIND without token = %d, want 401", code)
	}
	for _, path := range []string{"b/", "b/shelf.json", ""} {
		if code := lanGet(t, st, path, st.Token); code != http.StatusNotFound {
			t.Errorf("PROPFIND %q = %d, want 404", path, code)
		}
	}

	// Changing the selection applies without a restart, on the same port.
	st2, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"b"}})
	if err != nil {
		t.Fatal(err)
	}
	if st2.URL != st.URL {
		t.Errorf("URL changed with the selection: %q -> %q", st.URL, st2.URL)
	}
	if code := lanGet(t, st2, "a/", st.Token); code != http.StatusNotFound {
		t.Errorf("PROPFIND unticked shelf = %d, want 404", code)
	}
	if code := lanGet(t, st2, "b/", st.Token); code != http.StatusMultiStatus {
		t.Errorf("PROPFIND ticked shelf = %d, want 207", code)
	}

	addr := strings.TrimSuffix(strings.TrimPrefix(st.URL, "http://"), "/dav/")
	st3, err := env.share.apply(LANShareParams{Enabled: false, ShelfIDs: []string{"b"}})
	if err != nil {
		t.Fatal(err)
	}
	if st3.Running || st3.URL != "" || st3.Token != "" {
		t.Fatalf("status after disabling = %+v", st3)
	}
	// The port is released at once.
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		conn.Close()
		t.Fatalf("export still accepts connections on %s after disabling", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port not released: %v", err)
	}
	ln.Close()
}

func TestLANShareSurvivesRestart(t *testing.T) {
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"b"}})
	if err != nil {
		t.Fatal(err)
	}
	env.share.stop()

	// Written to the data root, not into a shelf.
	data, err := os.ReadFile(filepath.Join(env.dataRoot, lanShareConfigName))
	if err != nil {
		t.Fatal(err)
	}
	var saved lanShareConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled || len(saved.ShelfIDs) != 1 || saved.ShelfIDs[0] != "b" {
		t.Errorf("saved config = %+v", saved)
	}
	for id, root := range env.shelfRootsBy {
		for _, name := range []string{lanShareConfigName, lanShareTokenName} {
			if _, err := os.Stat(filepath.Join(root, name)); err == nil {
				t.Errorf("%s written into shelf %s", name, id)
			}
		}
	}

	again := env.reopen(t)
	again.startSaved()
	st2, err := again.status()
	if err != nil {
		t.Fatal(err)
	}
	if !st2.Running || st2.Token != st.Token {
		t.Fatalf("after restart: running=%v token changed=%v (%s)", st2.Running, st2.Token != st.Token, st2.Error)
	}
	if code := lanGet(t, st2, "b/", st.Token); code != http.StatusMultiStatus {
		t.Errorf("PROPFIND after restart = %d, want 207", code)
	}
}

func TestLANShareRefusesForeignHost(t *testing.T) {
	env := newLANShareEnv(t)
	for _, host := range []string{"0.0.0.0", "10.9.9.9"} {
		st, err := env.share.apply(LANShareParams{Enabled: true, Host: host})
		if err == nil || st.Running || st.Enabled {
			t.Errorf("host %s: err=%v status=%+v, want refused and off", host, err, st)
		}
	}
}

func TestLANShareStartSavedReportsMissingAddress(t *testing.T) {
	env := newLANShareEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	env.share.stop()

	// The network changed while the app was closed.
	lanAddresses = func() ([]string, error) { return []string{"192.0.2.7"}, nil }
	again := env.reopen(t)
	again.startSaved()
	st, err := again.status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.Running || st.Error == "" {
		t.Errorf("status = %+v, want enabled, not running, with an error", st)
	}
}

func TestLANShareRecoversFromGoneAddress(t *testing.T) {
	env := newLANShareEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	env.share.stop()
	// A new address, bound on loopback: 127.0.0.2 is not on every machine.
	lanAddresses = func() ([]string, error) { return []string{"192.0.2.7"}, nil }
	lanListen = func(network, addr string) (net.Listener, error) {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		return net.Listen(network, net.JoinHostPort("127.0.0.1", port))
	}
	again := env.reopen(t)
	again.startSaved()

	// The page offers the address that exists now, and switching back on
	// binds it, whether the page sends the saved host back or not.
	st, err := again.status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Host != "192.0.2.7" {
		t.Fatalf("status host = %q, want the current address", st.Host)
	}
	for _, host := range []string{"127.0.0.1", ""} {
		if _, err := again.apply(LANShareParams{Enabled: false}); err != nil {
			t.Fatal(err)
		}
		st, err = again.apply(LANShareParams{Enabled: true, Host: host})
		if err != nil {
			t.Fatalf("host %q: %v", host, err)
		}
		if !st.Running || st.Host != "192.0.2.7" {
			t.Errorf("host %q: status = %+v, want running on 192.0.2.7", host, st)
		}
	}
}

// failSave makes every lan_share.json write fail until the returned func runs.
func failSave(t *testing.T) (restore func()) {
	t.Helper()
	real := saveLANShare
	saveLANShare = func(string, lanShareConfig) error { return errors.New("disk full") }
	return func() { saveLANShare = real }
}

func lanDialable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func TestLANShareUnsavedEnableDoesNotOpen(t *testing.T) {
	env := newLANShareEnv(t)
	restore := failSave(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	restore()
	if err == nil {
		t.Fatal("apply succeeded with an unwritable config")
	}
	if st.Running || st.Enabled || env.share.isShared("a") {
		t.Errorf("status = %+v, want nothing applied", st)
	}
}

func TestLANShareUnsavedWideningDoesNotApply(t *testing.T) {
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	restore := failSave(t)
	_, err = env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a", "b"}})
	restore()
	if err == nil {
		t.Fatal("apply succeeded with an unwritable config")
	}
	if code := lanGet(t, st, "b/", st.Token); code != http.StatusNotFound {
		t.Errorf("PROPFIND unsaved shelf = %d, want 404", code)
	}
}

func TestLANShareUnsavedDisableStopsAndSaysSo(t *testing.T) {
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	addr := strings.TrimSuffix(strings.TrimPrefix(st.URL, "http://"), "/dav/")

	restore := failSave(t)
	if _, err := env.share.apply(LANShareParams{Enabled: false, ShelfIDs: []string{"a"}}); err == nil {
		t.Fatal("apply hid the failed save")
	}
	// Stopped at once regardless.
	if lanDialable(addr) {
		t.Fatal("export still listening after switching off")
	}
	// Still said on a later read, as the page would after a reload.
	st2, err := env.share.status()
	if err != nil {
		t.Fatal(err)
	}
	if st2.Running || !strings.Contains(st2.Error, "not saved") {
		t.Errorf("status = %+v, want stopped with the unsaved warning", st2)
	}

	restore()
	st3, err := env.share.apply(LANShareParams{Enabled: false, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if st3.Error != "" {
		t.Errorf("warning kept after a successful save: %q", st3.Error)
	}
	if saved, _ := loadLANShareConfig(filepath.Join(env.dataRoot, lanShareConfigName)); saved.Enabled {
		t.Error("lan_share.json still enabled after the retry")
	}
}

func TestLANShareRejectsUnknownShelf(t *testing.T) {
	env := newLANShareEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"nope"}}); err == nil {
		t.Fatal("apply accepted an unknown shelf")
	}
	if st, _ := env.share.status(); st.Running {
		t.Error("listener started for a refused selection")
	}
}

func TestLANShareForgetsRemovedShelf(t *testing.T) {
	env := newLANShareEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: false, ShelfIDs: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := env.share.forgetShelf("a"); err != nil {
		t.Fatal(err)
	}
	if env.share.isShared("a") || !env.share.isShared("b") {
		t.Error("forgetShelf did not drop only the removed shelf")
	}
	saved, err := loadLANShareConfig(filepath.Join(env.dataRoot, lanShareConfigName))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.ShelfIDs) != 1 || saved.ShelfIDs[0] != "b" {
		t.Errorf("saved shelf ids = %v, want [b]", saved.ShelfIDs)
	}
}

func TestLANShareRotateToken(t *testing.T) {
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := env.share.rotateToken()
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Token == "" || rotated.Token == st.Token {
		t.Fatalf("token not replaced: %q", rotated.Token)
	}
	if code := lanGet(t, rotated, "a/", st.Token); code != http.StatusUnauthorized {
		t.Errorf("old token = %d, want 401", code)
	}
	if code := lanGet(t, rotated, "a/", rotated.Token); code != http.StatusMultiStatus {
		t.Errorf("new token = %d, want 207", code)
	}
}

func TestDetectLANAddressesExcludesLoopbackAndAny(t *testing.T) {
	addrs, err := detectLANAddresses()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			t.Errorf("unexpected address %q", a)
		}
	}
}
