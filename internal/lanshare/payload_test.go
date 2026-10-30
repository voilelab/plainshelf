package lanshare

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestParseContract(t *testing.T) {
	valid := Payload{V: Version, URL: "http://192.168.1.20:20001/dav/", Token: "private-token", Shelves: []Shelf{{ID: "one", Name: "One", URL: "http://192.168.1.20:20001/dav/one/"}}}
	tests := []struct {
		name   string
		change func(*Payload)
		want   string
	}{
		{"valid", func(*Payload) {}, ""},
		{"multiple", func(p *Payload) { p.Shelves = append(p.Shelves, Shelf{ID: "two", Name: "Two", URL: p.URL + "two/"}) }, ""},
		{"empty selection", func(p *Payload) { p.Shelves = []Shelf{} }, ""},
		{"version", func(p *Payload) { p.V = 2 }, "version"},
		{"missing version", func(p *Payload) { p.V = 0 }, "version"},
		{"missing token", func(p *Payload) { p.Token = "" }, "token"},
		{"missing root", func(p *Payload) { p.URL = "" }, "url"},
		{"missing shelves", func(p *Payload) { p.Shelves = nil }, "shelves"},
		{"missing id", func(p *Payload) { p.Shelves[0].ID = "" }, "ids"},
		{"missing name", func(p *Payload) { p.Shelves[0].Name = " " }, "names"},
		{"missing shelf URL", func(p *Payload) { p.Shelves[0].URL = "" }, "url"},
		{"ftp root", func(p *Payload) { p.URL = "ftp://example.com/" }, "url"},
		{"ftp shelf", func(p *Payload) { p.Shelves[0].URL = "ftp://example.com/" }, "url"},
		{"relative", func(p *Payload) { p.Shelves[0].URL = "/dav/one/" }, "url"},
		{"userinfo", func(p *Payload) { p.URL = "http://user:private-token@example.com/" }, "url"},
		{"query", func(p *Payload) { p.Shelves[0].URL += "?token=private-token" }, "url"},
		{"fragment", func(p *Payload) { p.Shelves[0].URL += "#private-token" }, "url"},
		{"other origin", func(p *Payload) { p.Shelves[0].URL = "https://other.example/one/" }, "origin"},
		{"duplicate id", func(p *Payload) { p.Shelves = append(p.Shelves, p.Shelves[0]) }, "ids"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			p.Shelves = append([]Shelf{}, valid.Shelves...)
			tt.change(&p)
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "missing shelves" {
				raw = []byte(strings.Replace(string(raw), `"shelves":[]`, `"shelves":null`, 1))
			}
			got, err := Parse(string(raw))
			if tt.want == "" {
				if err != nil || got.V != Version {
					t.Fatalf("valid payload refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %s error, got %v", tt.want, err)
			}
			if got.Token != "" || got.Shelves != nil || strings.Contains(err.Error(), valid.Token) {
				t.Fatal("invalid payload exposed credentials or partial output")
			}
		})
	}
	for _, raw := range []string{"null", "[]", "{", `{"v":1,"token":"private-token","url":false}`, `{"v":1,"v":1}`} {
		if _, err := Parse(raw); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}
