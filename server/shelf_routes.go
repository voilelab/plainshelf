package server

import (
	"net/http"
	"strings"
)

type shelfRouteKind uint8

const (
	unclassifiedShelfRoute shelfRouteKind = iota
	shelfScanRoute
	bookDownloadRoute
)

func classifyShelfRoute(urlPath string) shelfRouteKind {
	rest, ok := strings.CutPrefix(urlPath, "/api/shelves/")
	if !ok {
		return unclassifiedShelfRoute
	}
	shelfID, tail, ok := strings.Cut(rest, "/")
	if !ok || shelfID == "" {
		return unclassifiedShelfRoute
	}
	if tail == "scans" {
		return shelfScanRoute
	}
	parts := strings.Split(tail, "/")
	if len(parts) == 3 && parts[0] == "books" && parts[1] != "" && parts[2] == "download" {
		return bookDownloadRoute
	}
	return unclassifiedShelfRoute
}

func isReadOnlySafeRequest(r *http.Request) bool {
	switch classifyShelfRoute(r.URL.Path) {
	case shelfScanRoute:
		return r.Method == http.MethodPost
	case bookDownloadRoute:
		return r.Method == http.MethodPut || r.Method == http.MethodDelete
	default:
		return false
	}
}

func isShelfScanRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && classifyShelfRoute(r.URL.Path) == shelfScanRoute
}
