package main

import (
	"strings"

	"github.com/voilelab/plainshelf/internal/lanshare"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type LANImportResult struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

func (a *DesktopApp) PreviewLANShareQR(text string) ([]lanshare.Shelf, error) {
	p, err := lanshare.Parse(text)
	if err != nil {
		return nil, err
	}
	return p.Shelves, nil
}

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
