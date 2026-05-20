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

func readSourceID(r *http.Request) (string, error) {
	sourceID := strings.TrimSpace(r.PathValue("source_id"))
	if sourceID == "" {
		sourceID = strings.TrimSpace(r.URL.Query().Get("source_id"))
	}

	if sourceID == "" {
		return "", errors.New("missing source_id")
	}

	decoded, err := url.PathUnescape(sourceID)
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
