// Package lanshare defines the v:1 QR contract shared by LAN share clients.
package lanshare

import (
	"encoding/json/v2"
	"net/url"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

const Version = 1

type Shelf struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Payload struct {
	V       int     `json:"v"`
	URL     string  `json:"url"`
	Token   string  `json:"token"`
	Shelves []Shelf `json:"shelves"`
}

// Parse validates the entire payload before returning anything usable. Errors
// name fields, never their values: a pasted payload contains a bearer secret.
func Parse(text string) (Payload, error) {
	var p Payload
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return Payload{}, util.NewError("invalid QR JSON")
	}
	if p.V != Version {
		return Payload{}, util.NewError("unsupported QR version (expected v:1)")
	}
	if strings.TrimSpace(p.Token) == "" || strings.ContainsAny(p.Token, "\r\n") {
		return Payload{}, util.NewError("QR token is missing or invalid")
	}
	root, ok := address(p.URL)
	if !ok {
		return Payload{}, util.NewError("QR url must be an HTTP(S) URL without credentials, query or fragment")
	}
	if p.Shelves == nil {
		return Payload{}, util.NewError("QR shelves is missing")
	}
	ids := map[string]bool{}
	for _, s := range p.Shelves {
		if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Name) == "" || ids[s.ID] {
			return Payload{}, util.NewError("QR shelves require unique ids and nonempty names")
		}
		ids[s.ID] = true
		u, ok := address(s.URL)
		if !ok || u.Scheme != root.Scheme || !strings.EqualFold(u.Host, root.Host) {
			return Payload{}, util.NewError("QR shelf url must be an HTTP(S) URL on the share origin without credentials, query or fragment")
		}
	}
	return p, nil
}

func address(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	return u, err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(raw, "#")
}
