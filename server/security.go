package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type SecurityMode string

const (
	SecurityModeUnset      SecurityMode = ""
	SecurityModeLocalToken SecurityMode = "local_token"
	SecurityModeNone       SecurityMode = "none"
	SecurityModePassword   SecurityMode = "password"
	SecurityModeExternal   SecurityMode = "external"

	defaultTokenHeader = "X-PlainShelf-Token"
)

type SecurityConf struct {
	Mode                        SecurityMode `yaml:"mode"`
	ProtectRead                 bool         `yaml:"protect_read"`
	TokenHeader                 string       `yaml:"token_header"`
	AllowMissingOriginWithToken *bool        `yaml:"allow_missing_origin_with_token"`
	AllowedOrigins              []string     `yaml:"allowed_origins"`
}

type Security struct {
	conf           SecurityConf
	token          string
	allowedOrigins map[string]struct{}
}

func NewSecurity(conf *SecurityConf) (*Security, error) {
	confValue := normalizeSecurityConf(conf)
	switch confValue.Mode {
	case SecurityModeLocalToken, SecurityModeNone:
	case SecurityModePassword, SecurityModeExternal:
		return nil, util.Errorf("security mode %q is reserved but not implemented yet", confValue.Mode)
	default:
		return nil, util.Errorf("unknown security mode %q", confValue.Mode)
	}
	sec := &Security{
		conf:           confValue,
		allowedOrigins: make(map[string]struct{}, len(confValue.AllowedOrigins)),
	}

	for _, origin := range confValue.AllowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return nil, util.Errorf("invalid security allowed_origin %q: %w", origin, err)
		}
		sec.allowedOrigins[normalized] = struct{}{}
	}

	if confValue.Mode == SecurityModeLocalToken {
		token, err := generateLocalToken()
		if err != nil {
			return nil, util.Errorf("generate local token: %w", err)
		}
		sec.token = token
	}

	return sec, nil
}

func normalizeSecurityConf(conf *SecurityConf) SecurityConf {
	confValue := SecurityConf{}
	if conf != nil {
		confValue = *conf
	}
	if confValue.Mode == SecurityModeUnset {
		confValue.Mode = SecurityModeLocalToken
	}
	if strings.TrimSpace(confValue.TokenHeader) == "" {
		confValue.TokenHeader = defaultTokenHeader
	}
	if confValue.Mode == SecurityModeLocalToken && confValue.AllowMissingOriginWithToken == nil {
		confValue.AllowMissingOriginWithToken = new(true)
	}
	if confValue.Mode == SecurityModeLocalToken && len(confValue.AllowedOrigins) == 0 {
		confValue.AllowedOrigins = defaultAllowedOrigins()
	}
	return confValue
}

func defaultAllowedOrigins() []string {
	return []string{
		"http://127.0.0.1:20000",
		"http://localhost:20000",
		"http://127.0.0.1:5173",
		"http://localhost:5173",
	}
}

func generateLocalToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}

func ValidateSecurityForListenAddr(conf *SecurityConf, listenAddr string) error {
	if conf != nil && conf.Mode != SecurityModeUnset {
		return nil
	}
	if isLoopbackListenAddr(listenAddr) {
		return nil
	}
	return util.Errorf("app_conf.security.mode must be set when server_conf.addr %q is not loopback", listenAddr)
}

// InsecureNetworkExposure reports mode none bound to a non-loopback address,
// which is what the Web UI's persistent "no API auth" warning keys on. A
// loopback bind is ordinary local development, not an exposure, and in-process
// embedders open no port at all.
func (sec *Security) InsecureNetworkExposure(listenAddr string) bool {
	return sec != nil && sec.conf.Mode == SecurityModeNone && !isLoopbackListenAddr(listenAddr)
}

func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// setStaticSecurityHeaders writes the browser-hardening headers sent on every
// response, whatever the security mode: nosniff stops a response being
// reinterpreted as another content type, DENY refuses to be framed so a
// clickjacking page cannot borrow the token-bearing UI, and no-referrer keeps
// the address — which can carry a book or shelf identifier — off any outbound
// request.
//
// None depend on the token gate, so they are set before it, ahead of any mode
// branching or early return. The document-level Content-Security-Policy belongs
// on the HTML response that carries the token; see spaHandlers.fallback.
func setStaticSecurityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
}

func (sec *Security) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setStaticSecurityHeaders(w.Header())

		if sec == nil || sec.conf.Mode == SecurityModeNone {
			next.ServeHTTP(w, r)
			return
		}

		sec.applyCORS(w, r)

		if r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api/") {
			if !sec.isAllowedRequestOrigin(r) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		switch {
		case sec.requiresToken(r):
			tokenOK := sec.validToken(r)
			if !tokenOK {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !sec.originAllowedForProtectedRequest(r, tokenOK) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		case sec.isTokenExemptScan(r):
			if !sec.originAllowedForTokenExemptRequest(r) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (sec *Security) requiresToken(r *http.Request) bool {
	if sec == nil || sec.conf.Mode == SecurityModeNone {
		return false
	}
	if r.URL.Path == "/health" {
		return false
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}
	if IsLogAPIPath(r.URL.Path) {
		return true
	}
	if sec.conf.ProtectRead {
		return true
	}
	if sec.isTokenExemptScan(r) {
		return false
	}
	return IsMutatingMethod(r.Method)
}

// isTokenExemptScan reports the one write-shaped request the token gate lets
// through: the shelf rescan, while protect_read is off. A rescan is a read — it
// walks the shelf and rebuilds the cache, which is why read-only mode already
// exempts it — and being a POST was the only thing holding it, which made the
// "refresh the book list" button fail with 401 under the shipped defaults.
//
// Exempt from the token is not exempt from CSRF: see
// originAllowedForTokenExemptRequest.
func (sec *Security) isTokenExemptScan(r *http.Request) bool {
	return sec != nil && sec.conf.Mode != SecurityModeNone && !sec.conf.ProtectRead &&
		isReadOnlySafeRequest(r)
}

// IsLogAPIPath reports the log API, which always needs a token. protect_read
// answers "must a reader authenticate to see the shelf", and the logs are not
// shelf content: they record every request path — and so the shelf's structure —
// with the access times and remote addresses behind it. Deliberately not a
// setting: a safe default is not a choice to offer.
func IsLogAPIPath(path string) bool {
	return path == "/api/logs" || strings.HasPrefix(path, "/api/logs/")
}

// IsMutatingMethod is the single definition of "this request writes". Both
// gates that care -- the token requirement here and the read-only rejection in
// Handler -- read it, so the two cannot drift apart into disagreeing about
// which methods are writes.
func IsMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (sec *Security) validToken(r *http.Request) bool {
	if sec.conf.Mode != SecurityModeLocalToken {
		return sec.conf.Mode == SecurityModeNone
	}
	if sec.token == "" {
		return false
	}
	for _, candidate := range []string{bearerToken(r.Header.Get("Authorization")), r.Header.Get(sec.conf.TokenHeader)} {
		if candidate == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(sec.token)) == 1 {
			return true
		}
	}
	return false
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func (sec *Security) originAllowedForProtectedRequest(r *http.Request, tokenOK bool) bool {
	origin, hasOrigin := sec.requestOrigin(r)
	if !hasOrigin {
		return tokenOK && sec.allowMissingOriginWithToken()
	}
	return sec.isAllowedOrigin(origin)
}

// originAllowedForTokenExemptRequest is the CSRF half of the gate for a request
// let through without a token. A browser attaches Origin to every cross-site
// POST, so an unknown origin is a page acting on its own. No origin at all is
// not a browser — the Android client's native HTTP bridge sends none — so there
// is nothing to forge. That is where it parts from
// originAllowedForProtectedRequest, which needs a token to vouch for a missing
// origin; demanding one here would restore the 401 this exemption removes.
func (sec *Security) originAllowedForTokenExemptRequest(r *http.Request) bool {
	origin, hasOrigin := sec.requestOrigin(r)
	if !hasOrigin {
		return true
	}
	return sec.isAllowedOrigin(origin)
}

func (sec *Security) isAllowedRequestOrigin(r *http.Request) bool {
	origin, ok := sec.requestOrigin(r)
	return ok && sec.isAllowedOrigin(origin)
}

func (sec *Security) requestOrigin(r *http.Request) (string, bool) {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return "", true
		}
		return normalized, true
	}

	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", true
		}
		normalized, err := normalizeOrigin(u.Scheme + "://" + u.Host)
		if err != nil {
			return "", true
		}
		return normalized, true
	}

	return "", false
}

func (sec *Security) isAllowedOrigin(origin string) bool {
	_, ok := sec.allowedOrigins[origin]
	return ok
}

func normalizeOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", util.Errorf("empty origin")
	}
	u, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", util.Errorf("origin must include scheme and host")
	}
	if u.Path != "" && u.Path != "/" {
		return "", util.Errorf("origin must not include path")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", util.Errorf("origin must not include user info, query, or fragment")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func (sec *Security) applyCORS(w http.ResponseWriter, r *http.Request) {
	originHeader := strings.TrimSpace(r.Header.Get("Origin"))
	if originHeader == "" {
		return
	}
	origin, err := normalizeOrigin(originHeader)
	if err != nil || !sec.isAllowedOrigin(origin) {
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", originHeader)
	h.Add("Vary", "Origin")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, "+sec.conf.TokenHeader)
	// The number a user quotes in a bug report, so a browser on an allowed origin
	// has to read it off the response. Only origins that already read the whole
	// body reach this far.
	h.Set("Access-Control-Expose-Headers", RequestIDHeader)
}

func (sec *Security) allowMissingOriginWithToken() bool {
	return sec.conf.AllowMissingOriginWithToken != nil && *sec.conf.AllowMissingOriginWithToken
}

func (sec *Security) Token() string {
	if sec == nil {
		return ""
	}
	return sec.token
}

func (sec *Security) TokenHeader() string {
	if sec == nil || strings.TrimSpace(sec.conf.TokenHeader) == "" {
		return defaultTokenHeader
	}
	return sec.conf.TokenHeader
}

func (sec *Security) IsEnabled() bool {
	return sec != nil && sec.conf.Mode != SecurityModeNone
}

func (sec *Security) LogStartup(logger *logutil.Logger) {
	if sec == nil {
		return
	}
	switch sec.conf.Mode {
	case SecurityModeLocalToken:
		logger.Info("Local token security enabled; mutating /api requests require token header or Authorization: Bearer token", "token_header", sec.TokenHeader())
	case SecurityModeNone:
		logger.Warn("PlainShelf API security is disabled by app_conf.security.mode=none")
	default:
		logger.Info("Security mode configured", "mode", sec.conf.Mode)
	}
}
