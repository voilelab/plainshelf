package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

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
		confValue.AllowMissingOriginWithToken = boolPtr(true)
	}
	if confValue.Mode == SecurityModeLocalToken && len(confValue.AllowedOrigins) == 0 {
		confValue.AllowedOrigins = defaultAllowedOrigins()
	}
	return confValue
}

func boolPtr(v bool) *bool {
	return &v
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

func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (sec *Security) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		if sec.requiresToken(r) {
			tokenOK := sec.validToken(r)
			if !tokenOK {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !sec.originAllowedForProtectedRequest(r, tokenOK) {
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
	if sec.conf.ProtectRead {
		return true
	}
	return isMutatingMethod(r.Method)
}

func isMutatingMethod(method string) bool {
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

func (sec *Security) LogStartup() {
	if sec == nil {
		return
	}
	switch sec.conf.Mode {
	case SecurityModeLocalToken:
		log.Printf("Local token security enabled; mutating /api requests require %s or Authorization: Bearer token", sec.TokenHeader())
	case SecurityModeNone:
		log.Printf("WARNING: PlainShelf API security is disabled by app_conf.security.mode=none")
	default:
		log.Printf("Security mode %q configured", sec.conf.Mode)
	}
}
