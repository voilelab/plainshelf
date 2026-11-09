package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

func readShelfID(r *http.Request) (string, error) {
	return decodePathID("shelf_id", r.PathValue("shelf_id"))
}

func readBookID(r *http.Request) (string, error) {
	bookID := r.PathValue("book_id")
	if strings.TrimSpace(bookID) == "" {
		bookID = r.URL.Query().Get("book_id")
	}
	return decodePathID("book_id", bookID)
}

func readSourceID(r *http.Request) (string, error) {
	return decodePathID("source_id", r.PathValue("source_id"))
}

// readAssetName deliberately does not url.PathUnescape, unlike its neighbours
// here: ServeMux has already unescaped the wildcard, so decoding a second time
// rewrites any name that legitimately contains a percent escape. A file named
// "chart%20one.png" would be read as "chart one.png" and quietly serve a
// different file.

func readAssetName(r *http.Request) (string, error) {

	assetName := r.PathValue("asset_name")
	if assetName == "" {
		return "", errors.New("missing asset_name")
	}

	return assetName, nil
}

func readLogID(r *http.Request) (string, error) {
	return decodePathID("log_id", r.PathValue("log_id"))
}

func readTaskChainID(r *http.Request) (string, error) {
	return decodePathID("taskchain_id", r.PathValue("taskchain_id"))
}

func decodePathID(name, raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", errors.New("missing " + name)
	}

	decoded, err := url.PathUnescape(id)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

func readFolderParts(r *http.Request) ([]string, error) {
	rawFolder := strings.TrimSpace(r.PathValue("folder_path"))

	decoded, err := url.PathUnescape(rawFolder)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	parts := strings.Split(decoded, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}
