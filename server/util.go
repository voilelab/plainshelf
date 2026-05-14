package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

func readBookID(r *http.Request) (string, error) {
	bookID := strings.TrimSpace(r.PathValue("book_id"))
	if bookID == "" {
		bookID = strings.TrimSpace(r.URL.Query().Get("book_id"))
	}
	if bookID == "" {
		return "", errors.New("missing book_id")
	}

	decoded, err := url.PathUnescape(bookID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

func readSnapshotID(r *http.Request) (string, error) {
	snapshotID := strings.TrimSpace(r.PathValue("snapshot_id"))
	if snapshotID == "" {
		snapshotID = strings.TrimSpace(r.URL.Query().Get("snapshot_id"))
	}
	if snapshotID == "" {
		return "", errors.New("missing snapshot_id")
	}

	decoded, err := url.PathUnescape(snapshotID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

func readLayerParts(r *http.Request) ([]string, error) {
	rawLayer := strings.TrimSpace(r.PathValue("layer_path"))

	decoded, err := url.PathUnescape(rawLayer)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	parts := strings.Split(decoded, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}
