package main

import (
	"strings"

	"github.com/voilelab/plainshelf/internal/lanshare"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// LANImportResult intentionally contains no payload text or credential.
type LANImportResult struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

// PreviewLANShareQR validates without connecting or storing the secret.
func (a *DesktopApp) PreviewLANShareQR(text string) ([]lanshare.Shelf, error) {
	p, err := lanshare.Parse(text)
	if err != nil {
		return nil, err
	}
	return p.Shelves, nil
}

// ImportLANShareQR is called only after the user confirms the preview. Each
// source is independent; a failed check saves nothing for that source, and a
// retry skips sources already present without replacing their credentials.
func (a *DesktopApp) ImportLANShareQR(text string) ([]LANImportResult, error) {
	p, err := lanshare.Parse(text)
	if err != nil {
		return nil, err
	}
	if a.app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}
	results := make([]LANImportResult, 0, len(p.Shelves))
	for _, s := range p.Shelves {
		conf, err := loadDesktopShelves(a.shelvesConfigPath)
		if err != nil {
			return results, util.NewError("cannot load shelf configuration")
		}
		result := LANImportResult{Name: s.Name, URL: s.URL}
		duplicate := false
		for _, e := range conf.VirtualShelves {
			if e.Source != nil && e.Source.Type == shelf.SourcePlainShelf && strings.TrimRight(e.Source.URL, "/") == strings.TrimRight(s.URL, "/") {
				duplicate = true
				break
			}
		}
		if duplicate {
			result.Status = "exists"
		} else {
			remote := ShelfSourceParams{Source: shelf.SourcePlainShelf, URL: s.URL, Secret: p.Token}
			check, err := a.CheckShelfSource(remote)
			switch {
			case err != nil:
				result.Status = "failed"
			case check.Code != "":
				result.Status = check.Code
			default:
				if err := a.AddShelf(AddShelfParams{Name: s.Name, Remote: remote, ReadOnly: true}); err != nil {
					// Do not return arbitrary transport or keychain errors with secret data.
					result.Status = "failed"
				} else {
					result.Status = "added"
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}
