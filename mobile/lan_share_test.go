package mobile

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

// shareOnLoopback lets the share bind 127.0.0.1 on a free port.
func shareOnLoopback(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	prevPort, prevAllowed := lanSharePort, lanHostAllowed
	lanSharePort = port
	lanHostAllowed = func(ip netip.Addr) bool { return ip.IsLoopback() }
	t.Cleanup(func() { lanSharePort, lanHostAllowed = prevPort, prevAllowed })
}

func startShare(t *testing.T, ids ...string) lanShareState {
	t.Helper()
	idsJSON, _ := json.Marshal(ids)
	out, err := StartLANShare("127.0.0.1", string(idsJSON))
	if err != nil {
		t.Fatalf("StartLANShare: %v", err)
	}
	var st lanShareState
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func davDo(t *testing.T, method, url, token string) int {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method == "PROPFIND" {
		req.Header.Set("Depth", "1")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return res.StatusCode
}

func dialable(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		_ = c.Close()
	}
	return err == nil
}

func TestLANShareIsOffUntilStarted(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	createShelf(t, "Phone")

	out, err := LANShareStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"running":false`) {
		t.Fatalf("status before start: %s", out)
	}
	if got := LANShareIdleMillis(); got != -1 {
		t.Fatalf("idle before start = %d", got)
	}
	if dialable(net.JoinHostPort("127.0.0.1", strconv.Itoa(lanSharePort))) {
		t.Fatal("something listens before the share started")
	}
}

func TestLANShareServesOnlyTokenReads(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	shared := createShelf(t, "Shared")
	other := createShelf(t, "Private")

	st := startShare(t, shared)
	if !st.Running || st.Token == "" || !strings.HasPrefix(st.URL, "http://127.0.0.1:") {
		t.Fatalf("status: %+v", st)
	}

	if got := davDo(t, "PROPFIND", st.URL+shared+"/", ""); got != http.StatusUnauthorized {
		t.Errorf("without token: %d", got)
	}
	if got := davDo(t, "PROPFIND", st.URL+shared+"/", "wrong"); got != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", got)
	}
	if got := davDo(t, "PROPFIND", st.URL+shared+"/", st.Token); got != http.StatusMultiStatus {
		t.Errorf("with token: %d", got)
	}
	if got := davDo(t, "PROPFIND", st.URL+other+"/", st.Token); got != http.StatusNotFound {
		t.Errorf("unshared shelf: %d", got)
	}
	for _, m := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "PROPPATCH"} {
		for _, token := range []string{"", st.Token} {
			if got := davDo(t, m, st.URL+shared+"/shelf.json", token); got != http.StatusMethodNotAllowed {
				t.Errorf("%s (token %t): %d", m, token != "", got)
			}
		}
	}
}

func TestLANShareStopClosesTheListener(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")
	st := startShare(t, id)
	addr := strings.TrimSuffix(strings.TrimPrefix(st.URL, "http://"), "/dav/")
	if !dialable(addr) {
		t.Fatal("share is not listening")
	}

	if err := StopLANShare(); err != nil {
		t.Fatal(err)
	}
	if dialable(addr) {
		t.Fatal("still listening after StopLANShare")
	}
	if got := LANShareIdleMillis(); got != -1 {
		t.Fatalf("idle after stop = %d", got)
	}

	startShare(t, id)
	if err := Stop(); err != nil {
		t.Fatal(err)
	}
	if dialable(addr) {
		t.Fatal("still listening after Stop")
	}
	if err := StopLANShare(); err != nil {
		t.Fatalf("StopLANShare after Stop: %v", err)
	}
}

func TestLANShareTokenSurvivesRestart(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")
	first := startShare(t, id)
	if err := StopLANShare(); err != nil {
		t.Fatal(err)
	}
	if again := startShare(t, id); again.Token != first.Token {
		t.Fatal("token changed between shares; a device that saved it would be locked out")
	}
}

// Only requests the token admits count as activity, so a device without it
// cannot hold the share open.
func TestLANShareIdleCountsAdmittedRequests(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")
	st := startShare(t, id)

	time.Sleep(50 * time.Millisecond)
	davDo(t, "PROPFIND", st.URL+id+"/", "")
	davDo(t, http.MethodPut, st.URL+id+"/x", st.Token)
	if got := LANShareIdleMillis(); got < 50 {
		t.Fatalf("refused requests reset idle: %d ms", got)
	}
	davDo(t, "PROPFIND", st.URL+id+"/", st.Token)
	if got := LANShareIdleMillis(); got >= 50 {
		t.Fatalf("admitted request did not reset idle: %d ms", got)
	}
}

func TestLANShareSelection(t *testing.T) {
	shareOnLoopback(t)
	startT(t, t.TempDir())
	a := createShelf(t, "A")
	b := createShelf(t, "B")

	if _, err := StartLANShare("127.0.0.1", `[]`); err == nil {
		t.Fatal("started with no shelf")
	}
	if _, err := StartLANShare("127.0.0.1", `["missing"]`); err == nil {
		t.Fatal("started with an unknown shelf")
	}
	st := startShare(t, a)
	// Same host: the shelves change, the listener does not.
	if again := startShare(t, b); again.URL != st.URL || len(again.ShelfIDs) != 1 || again.ShelfIDs[0] != b {
		t.Fatalf("after reselecting: %+v", again)
	}
	if got := davDo(t, "PROPFIND", st.URL+a+"/", st.Token); got != http.StatusNotFound {
		t.Errorf("deselected shelf: %d", got)
	}

	if err := RemoveShelf(b); err != nil {
		t.Fatal(err)
	}
	out, _ := LANShareStatus()
	if strings.Contains(out, b) {
		t.Fatalf("removed shelf still shared: %s", out)
	}
}

func TestLANShareRefusesNonLANHosts(t *testing.T) {
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")
	for _, host := range []string{"0.0.0.0", "127.0.0.1", "::1", "fe80::1", "224.0.0.1", "not-an-ip", ""} {
		if _, err := StartLANShare(host, `["`+id+`"]`); err == nil {
			_ = StopLANShare()
			t.Errorf("bound %q", host)
		}
	}
}

func TestLANShareNotStarted(t *testing.T) {
	if _, err := StartLANShare("192.168.1.2", `[]`); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("StartLANShare before Start: %v", err)
	}
	if err := StopLANShare(); err != nil {
		t.Fatalf("StopLANShare before Start: %v", err)
	}
	if got := LANShareIdleMillis(); got != -1 {
		t.Fatalf("idle before Start = %d", got)
	}
}
