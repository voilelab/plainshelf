package main

import (
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
	"github.com/voilelab/plainshelf/internal/lanshareqr"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
)

const (
	lanShareConfigName = "lan_share.json"
	lanShareTokenName  = "lan_share.token"
)

var lanShareDefaultPort = 20001

var lanAddresses = detectLANAddresses

var (
	lanListen    = net.Listen
	saveLANShare = saveLANShareConfig
)

type lanShareConfig struct {
	Enabled  bool     `json:"enabled,omitzero"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitzero"`
	ShelfIDs []string `json:"shelf_ids"`
}

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

	Running   bool            `json:"running"`
	Host      string          `json:"host"`
	Addresses []string        `json:"addresses"`
	Shelves   []LANShareShelf `json:"shelves"`

	URL    string `json:"url"`
	Token  string `json:"token"`
	QRCode string `json:"qr_code"`
	Error  string `json:"error"`
}

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

	requested, bound string

	startErr, saveErr string

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

		if next.Host == s.conf.Host {
			next.Host = currentLANHost(next.Host)
		}

		if widens(s.conf, next) {
			if err := saveLANShare(s.configPath, next); err != nil {
				return s.statusLocked(shelves), util.Errorf("saving LAN share config: %w", err)
			}
		}
		listenErr = s.listen(next.Host)
		if listenErr != nil {

			next.Enabled = false
			s.close()
		}
	} else {
		s.close()
	}

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

func (s *lanShare) saveApplied() error {
	if err := saveLANShare(s.configPath, s.conf); err != nil {
		s.saveErr = "LAN share settings were not saved, so the next start may share as before: " + err.Error()
		return util.Errorf("saving LAN share config: %w", err)
	}
	s.saveErr = ""
	return nil
}

func widens(cur, next lanShareConfig) bool {
	if !next.Enabled {
		return false
	}
	if !cur.Enabled || cur.Host != next.Host {
		return true
	}
	return slices.ContainsFunc(next.ShelfIDs, func(id string) bool { return !slices.Contains(cur.ShelfIDs, id) })
}

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
	return lanshareqr.Encode(payload)
}

func currentLANHost(host string) string {
	addrs, err := lanAddresses()
	if err != nil || len(addrs) == 0 || slices.Contains(addrs, host) {
		return host
	}
	return addrs[0]
}

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

func (a *DesktopApp) GetLANShare() (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.status()
}

func (a *DesktopApp) SetLANShare(params LANShareParams) (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.apply(params)
}

func (a *DesktopApp) RotateLANShareToken() (LANShareStatus, error) {
	if a.lanShare == nil {
		return LANShareStatus{}, util.NewError("desktop backend app instance is nil")
	}
	return a.lanShare.rotateToken()
}
