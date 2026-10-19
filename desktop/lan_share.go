package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/lanshare"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"rsc.io/qr"
)

// The desktop UI reaches Go in-process; this export is the only listener the
// desktop ever opens, and only while the user has it switched on.

const (
	lanShareConfigName = "lan_share.json"
	lanShareTokenName  = "lan_share.token"
)

// lanShareDefaultPort is used unless lan_share.json names a port; a var so
// tests can ask for any free one.
var lanShareDefaultPort = 20001

// lanAddresses lists the addresses the export may bind; a var for tests.
var lanAddresses = detectLANAddresses

// lanListen and saveLANShare are vars so tests can stand in for the network
// and the disk.
var (
	lanListen    = net.Listen
	saveLANShare = saveLANShareConfig
)

// lanShareConfig is lan_share.json in the desktop data root, outside every
// shelf.
type lanShareConfig struct {
	Enabled  bool     `json:"enabled,omitzero"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitzero"`
	ShelfIDs []string `json:"shelf_ids"`
}

// LANShareParams is what the settings page submits; see AddShelfParams for
// why bindings take a struct.
type LANShareParams struct {
	Enabled  bool     `json:"enabled"`
	Host     string   `json:"host"`
	ShelfIDs []string `json:"shelfIDs"`
}

type LANShareShelf struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Shared bool   `json:"shared"`
}

type LANShareStatus struct {
	Enabled bool `json:"enabled"`
	// Running is false while enabled when the listener could not start; Error
	// says why.
	Running   bool            `json:"running"`
	Host      string          `json:"host"`
	Addresses []string        `json:"addresses"`
	Shelves   []LANShareShelf `json:"shelves"`
	// URL, Token and QRCode are set only while running. QRCode is a PNG data
	// URL of lanShareQRPayload.
	URL    string `json:"url"`
	Token  string `json:"token"`
	QRCode string `json:"qr_code"`
	Error  string `json:"error"`
}

// lanShareQRVersion is lanShareQRPayload's "v". A reader refuses any other
// value rather than guess at the fields.
const lanShareQRVersion = lanshare.Version

type lanShareQRPayload = lanshare.Payload
type lanShareQRShelf = lanshare.Shelf

type lanShare struct {
	app               *server.App
	configPath        string
	tokenFile         string
	shelvesConfigPath string

	mu   sync.Mutex
	conf lanShareConfig
	srv  *http.Server
	// requested is the host:port srv was asked for, bound what it got.
	requested, bound string
	// startErr says why the saved share is not running; saveErr that what
	// runs differs from lan_share.json, which the next start would follow.
	startErr, saveErr string

	// shared is read by the handler on every request, so a changed selection
	// applies without restarting the listener.
	shared atomic.Pointer[map[string]bool]
}

func newLANShare(app *server.App, dataRoot, shelvesConfigPath string) *lanShare {
	s := &lanShare{
		app:               app,
		configPath:        filepath.Join(dataRoot, lanShareConfigName),
		tokenFile:         filepath.Join(dataRoot, lanShareTokenName),
		shelvesConfigPath: shelvesConfigPath,
	}
	conf, err := loadLANShareConfig(s.configPath)
	if err != nil {
		// Not fatal: the app runs with sharing off, and the next save
		// replaces the file.
		log.Println("Failed to load LAN share config:", err)
		s.startErr = err.Error()
		conf = lanShareConfig{}
	}
	s.conf = conf
	s.setShared(conf.ShelfIDs)
	return s
}

func loadLANShareConfig(path string) (lanShareConfig, error) {
	var conf lanShareConfig
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return conf, nil
	}
	if err != nil {
		return conf, util.Errorf("%w", err)
	}
	if err := json.Unmarshal(data, &conf); err != nil {
		return lanShareConfig{}, util.Errorf("%w", err)
	}
	return conf, nil
}

func saveLANShareConfig(path string, conf lanShareConfig) error {
	if conf.ShelfIDs == nil {
		conf.ShelfIDs = []string{}
	}
	data, err := json.Marshal(conf, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	return fsutil.WriteTextFileAtomic(path, lanShareConfigName+".*.tmp", string(data))
}

func (s *lanShare) setShared(ids []string) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	s.shared.Store(&m)
}

func (s *lanShare) isShared(shelfID string) bool {
	return (*s.shared.Load())[shelfID]
}

func (s *lanShare) port() int {
	if s.conf.Port != 0 {
		return s.conf.Port
	}
	return lanShareDefaultPort
}

// startSaved restores the state the app was closed in. A failure is kept for
// the settings page rather than stopping the app.
func (s *lanShare) startSaved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.conf.Enabled {
		return
	}
	if err := s.listen(s.conf.Host); err != nil {
		log.Println("Failed to start LAN share:", err)
		s.startErr = err.Error()
	}
}

// listen (re)binds the export to host; s.mu is held.
func (s *lanShare) listen(host string) error {
	addrs, err := lanAddresses()
	if err != nil {
		return err
	}
	if !slices.Contains(addrs, host) {
		return util.Errorf("address %q is not a LAN address of this computer", host)
	}
	if err := server.EnsureDAVExportToken(s.tokenFile); err != nil {
		return err
	}
	addr := net.JoinHostPort(host, strconv.Itoa(s.port()))
	if s.srv != nil && s.requested == addr {
		return nil
	}
	s.close()

	ln, err := lanListen("tcp", addr)
	if err != nil {
		return util.Errorf("%w", err)
	}
	srv := &http.Server{
		Handler:           s.app.DAVExportShelvesHandler(s.tokenFile, s.isShared),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		// Generous: a large book over Wi-Fi.
		WriteTimeout: 300 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println("LAN share stopped:", err)
		}
	}()
	s.srv = srv
	s.requested, s.bound = addr, ln.Addr().String()
	return nil
}

// close stops the listener at once, open transfers included; s.mu is held.
func (s *lanShare) close() {
	if s.srv == nil {
		return
	}
	if err := s.srv.Close(); err != nil {
		log.Println("Failed to close LAN share:", err)
	}
	s.srv = nil
	s.requested, s.bound = "", ""
}

func (s *lanShare) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.close()
}

func (s *lanShare) apply(params LANShareParams) (LANShareStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	shelves, err := loadDesktopShelves(s.shelvesConfigPath)
	if err != nil {
		return LANShareStatus{}, util.Errorf("loading shelf config: %w", err)
	}
	ids := []string{}
	for _, id := range params.ShelfIDs {
		id = strings.TrimSpace(id)
		if slices.Contains(ids, id) {
			continue
		}
		if !slices.ContainsFunc(shelves.Shelves, func(e desktopShelfEntry) bool { return e.ID == id }) {
			return LANShareStatus{}, util.Errorf("shelf with ID %q not found", id)
		}
		ids = append(ids, id)
	}

	next := s.conf
	next.ShelfIDs = ids
	next.Enabled = params.Enabled
	if host := strings.TrimSpace(params.Host); host != "" {
		next.Host = host
	}

	var listenErr error
	if next.Enabled {
		// The saved host may be gone after a network change; only a host the
		// user just picked is refused instead of replaced.
		if next.Host == s.conf.Host {
			next.Host = currentLANHost(next.Host)
		}
		// Saved before it applies, so a share that cannot be saved never
		// opens: neither now nor, unasked, at the next start.
		if widens(s.conf, next) {
			if err := saveLANShare(s.configPath, next); err != nil {
				return s.statusLocked(shelves), util.Errorf("saving LAN share config: %w", err)
			}
		}
		listenErr = s.listen(next.Host)
		if listenErr != nil {
			// Saved off, so a restart does not retry what just failed.
			next.Enabled = false
			s.close()
		}
	} else {
		s.close()
	}

	// Narrowing applies at once, saved or not; a failed save is kept on the
	// page until one succeeds.
	s.setShared(next.ShelfIDs)
	s.conf = next
	s.startErr = ""
	if err := s.saveApplied(); err != nil {
		return s.statusLocked(shelves), err
	}
	if listenErr != nil {
		return s.statusLocked(shelves), util.Errorf("starting LAN share: %w", listenErr)
	}
	return s.statusLocked(shelves), nil
}

func (s *lanShare) status() (LANShareStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	shelves, err := loadDesktopShelves(s.shelvesConfigPath)
	if err != nil {
		return LANShareStatus{}, util.Errorf("loading shelf config: %w", err)
	}
	return s.statusLocked(shelves), nil
}

func (s *lanShare) statusLocked(shelves *desktopShelvesConfig) LANShareStatus {
	st := LANShareStatus{
		Enabled:   s.conf.Enabled,
		Running:   s.srv != nil,
		Host:      s.conf.Host,
		Addresses: []string{},
		Shelves:   []LANShareShelf{},
		Error:     strings.Join(slices.DeleteFunc([]string{s.saveErr, s.startErr}, func(e string) bool { return e == "" }), "\n"),
	}
	if addrs, err := lanAddresses(); err == nil {
		st.Addresses = addrs
	}
	if !st.Running {
		// What the next switch-on binds, so the page never offers a gone address.
		st.Host = currentLANHost(st.Host)
	}
	for _, e := range shelves.Shelves {
		st.Shelves = append(st.Shelves, LANShareShelf{ID: e.ID, Name: e.Name, Shared: s.isShared(e.ID)})
	}
	if !st.Running {
		return st
	}

	st.URL = "http://" + s.bound + "/dav/"
	token, err := os.ReadFile(s.tokenFile)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.Token = strings.TrimSpace(string(token))
	qrCode, err := lanShareQRCode(newLANShareQRPayload(st))
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.QRCode = qrCode
	return st
}

func (s *lanShare) rotateToken() (LANShareStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := server.RotateDAVExportToken(s.tokenFile); err != nil {
		return LANShareStatus{}, err
	}
	shelves, err := loadDesktopShelves(s.shelvesConfigPath)
	if err != nil {
		return LANShareStatus{}, util.Errorf("loading shelf config: %w", err)
	}
	return s.statusLocked(shelves), nil
}

// forgetShelf drops a removed shelf, so a later shelf given the same id is not
// shared without being ticked.
func (s *lanShare) forgetShelf(shelfID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.conf.ShelfIDs, shelfID) {
		return nil
	}
	s.conf.ShelfIDs = slices.DeleteFunc(slices.Clone(s.conf.ShelfIDs), func(id string) bool { return id == shelfID })
	s.setShared(s.conf.ShelfIDs)
	return s.saveApplied()
}

// saveApplied writes s.conf, which is already in effect; s.mu is held.
func (s *lanShare) saveApplied() error {
	if err := saveLANShare(s.configPath, s.conf); err != nil {
		s.saveErr = "LAN share settings were not saved, so the next start may share as before: " + err.Error()
		return util.Errorf("saving LAN share config: %w", err)
	}
	s.saveErr = ""
	return nil
}

// widens reports whether next shares anything cur does not: switching on, a
// different address, or another shelf.
func widens(cur, next lanShareConfig) bool {
	if !next.Enabled {
		return false
	}
	if !cur.Enabled || cur.Host != next.Host {
		return true
	}
	return slices.ContainsFunc(next.ShelfIDs, func(id string) bool { return !slices.Contains(cur.ShelfIDs, id) })
}

// newLANShareQRPayload builds the payload of a running status.
func newLANShareQRPayload(st LANShareStatus) lanShareQRPayload {
	p := lanShareQRPayload{V: lanShareQRVersion, URL: st.URL, Token: st.Token, Shelves: []lanShareQRShelf{}}
	for _, sh := range st.Shelves {
		if sh.Shared {
			p.Shelves = append(p.Shelves, lanShareQRShelf{ID: sh.ID, Name: sh.Name, URL: st.URL + url.PathEscape(sh.ID) + "/"})
		}
	}
	return p
}

func lanShareQRCode(payload lanShareQRPayload) (string, error) {
	text, err := json.Marshal(payload, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	code, err := qr.Encode(string(text), qr.M)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("data:image/png;base64,")
	buf.WriteString(base64.StdEncoding.EncodeToString(code.PNG()))
	return buf.String(), nil
}

// currentLANHost returns host while it is still a LAN address, else the first
// one there is (or host, when there is none).
func currentLANHost(host string) string {
	addrs, err := lanAddresses()
	if err != nil || len(addrs) == 0 || slices.Contains(addrs, host) {
		return host
	}
	return addrs[0]
}

// detectLANAddresses lists this computer's IPv4 unicast addresses on up,
// non-loopback interfaces, private ranges first. Never 0.0.0.0: the export
// binds one network, not every one.
func detectLANAddresses() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	var private, other []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || !ip.IsGlobalUnicast() {
				continue
			}
			if ip.IsPrivate() {
				private = append(private, ip.String())
			} else {
				other = append(other, ip.String())
			}
		}
	}
	return slices.Compact(append(private, other...)), nil
}

// GetLANShare reports the LAN share switch, the shelves it may export, and
// while running the URL, token and QR code another device opens it with.
func (a *DesktopApp) GetLANShare() (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.status()
}

// SetLANShare switches the LAN share and sets what it exports. It is a Wails
// binding rather than an HTTP route, so nothing on the network can turn it on.
func (a *DesktopApp) SetLANShare(params LANShareParams) (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.apply(params)
}

// RotateLANShareToken replaces the token; the old one stops working at once.
func (a *DesktopApp) RotateLANShareToken() (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.rotateToken()
}
