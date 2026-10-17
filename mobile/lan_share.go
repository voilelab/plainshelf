package mobile

import (
	"encoding/json/v2"
	"errors"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
)

// The LAN share is the only listener the app ever opens, and only while the
// native foreground service runs it. Nothing about it is saved but the token,
// so a killed app comes back not sharing.

const lanShareTokenName = "lan_share.token"

// lanSharePort is the export's port, the desktop's default too; a var so tests
// can ask for any free one.
var lanSharePort = 20001

// lanHostAllowed reports whether the export may bind ip; a var so tests can
// bind loopback.
var lanHostAllowed = func(ip netip.Addr) bool {
	return ip.Is4() && ip.IsGlobalUnicast()
}

type lanShare struct {
	srv *http.Server
	// ln is closed here too: srv.Close misses it until Serve has started.
	ln net.Listener
	// host is what srv was asked for, url what another device opens.
	host, url string
	// shared is read on every request, so a changed selection applies without
	// rebinding.
	shared atomic.Pointer[map[string]bool]
	// lastActive is the unix nanos of the last request the token admitted.
	lastActive atomic.Int64
}

func (s *lanShare) setShared(ids []string) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	s.shared.Store(&m)
}

func (s *lanShare) isShared(id string) bool { return (*s.shared.Load())[id] }

func (s *lanShare) sharedIDs() []string {
	ids := []string{}
	for id := range *s.shared.Load() {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// lanShareState is what LANShareStatus reports, as JSON.
type lanShareState struct {
	Running  bool     `json:"running"`
	URL      string   `json:"url"`
	Token    string   `json:"token"`
	ShelfIDs []string `json:"shelf_ids"`
}

// StartLANShare exports the local shelves named in shelfIDsJSON (a JSON array)
// read-only over WebDAV on host, one of this device's LAN IPv4 addresses. A
// share already on host only changes its shelves. Returns LANShareStatus.
func StartLANShare(host, shelfIDsJSON string) (status string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	var ids []string
	if err := json.Unmarshal([]byte(shelfIDsJSON), &ids); err != nil {
		return "", util.Errorf("shelf ids: %w", err)
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil || !lanHostAllowed(ip) {
		return "", util.Errorf("address %q is not a LAN address", host)
	}

	if err := rt.lock(); err != nil {
		return "", err
	}
	defer rt.mu.Unlock()
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	if len(ids) == 0 {
		return "", util.Errorf("no shelf to share")
	}
	for _, id := range ids {
		if _, err := rt.find(id); err != nil {
			return "", err
		}
	}
	tokenFile := filepath.Join(rt.dataDir, lanShareTokenName)
	if err := server.EnsureDAVExportToken(tokenFile); err != nil {
		return "", err
	}

	if s := rt.share; s != nil && s.host == ip.String() {
		s.setShared(ids)
		return rt.lanShareStatusLocked()
	}
	rt.closeLANShare()

	ln, err := net.Listen("tcp", net.JoinHostPort(ip.String(), strconv.Itoa(lanSharePort)))
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	s := &lanShare{ln: ln, host: ip.String(), url: "http://" + ln.Addr().String() + "/dav/"}
	s.setShared(ids)
	s.lastActive.Store(time.Now().UnixNano())
	export := rt.app.DAVExportShelvesHandler(tokenFile, s.isShared)
	s.srv = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			aw := &activityWriter{ResponseWriter: w, share: s}
			export.ServeHTTP(aw, r)
			// Again at the end, so a long transfer counts until it finishes.
			aw.mark()
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		// Generous: a large book over Wi-Fi.
		WriteTimeout: 300 * time.Second,
	}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println("LAN share stopped:", err)
		}
	}()
	rt.share = s
	return rt.lanShareStatusLocked()
}

// StopLANShare closes the listener at once, open transfers included.
func StopLANShare() (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if errors.Is(err, ErrNotStarted) {
		return nil
	}
	if err != nil {
		return err
	}
	if rt.lock() != nil {
		return nil // Stop closed it.
	}
	defer rt.mu.Unlock()
	rt.closeLANShare()
	return nil
}

// closeLANShare stops the share; rt.mu is held exclusively.
func (rt *instance) closeLANShare() {
	if rt.share == nil {
		return
	}
	if err := rt.share.srv.Close(); err != nil {
		log.Println("Failed to close LAN share:", err)
	}
	_ = rt.share.ln.Close()
	rt.share = nil
}

// LANShareStatus reports the share as lanShareState JSON; URL and Token are
// set only while it runs.
func LANShareStatus() (status string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	if err := rt.rlock(); err != nil {
		return "", err
	}
	defer rt.mu.RUnlock()
	return rt.lanShareStatusLocked()
}

func (rt *instance) lanShareStatusLocked() (string, error) {
	st := lanShareState{ShelfIDs: []string{}}
	if s := rt.share; s != nil {
		token, err := os.ReadFile(filepath.Join(rt.dataDir, lanShareTokenName))
		if err != nil {
			return "", util.Errorf("%w", err)
		}
		st = lanShareState{Running: true, URL: s.url, Token: strings.TrimSpace(string(token)), ShelfIDs: s.sharedIDs()}
	}
	bs, err := json.Marshal(st, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// LANShareIdleMillis is how long the share has gone without a request the
// token admitted, or -1 when it is not running. The service closes it past
// its idle limit.
func LANShareIdleMillis() int64 {
	rt, err := running()
	if err != nil {
		return -1
	}
	if rt.rlock() != nil {
		return -1
	}
	defer rt.mu.RUnlock()
	if rt.share == nil {
		return -1
	}
	return time.Since(time.Unix(0, rt.share.lastActive.Load())).Milliseconds()
}

// forgetShared drops a removed shelf from the share; rt.mu is held.
func (rt *instance) forgetShared(id string) {
	if s := rt.share; s != nil && s.isShared(id) {
		s.setShared(slices.DeleteFunc(s.sharedIDs(), func(x string) bool { return x == id }))
	}
}

// activityWriter marks the share active when a response starts below 400, so
// a device without the token cannot keep the share open.
type activityWriter struct {
	http.ResponseWriter
	share       *lanShare
	written, ok bool
}

func (w *activityWriter) WriteHeader(status int) {
	if !w.written {
		w.written = true
		w.ok = status < http.StatusBadRequest
		w.mark()
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *activityWriter) mark() {
	if w.ok {
		w.share.lastActive.Store(time.Now().UnixNano())
	}
}

func (w *activityWriter) Write(p []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *activityWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
