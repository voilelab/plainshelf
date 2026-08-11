package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

func readShelfID(r *http.Request) (string, error) {
	shelfID := strings.TrimSpace(r.PathValue("shelf_id"))
	if shelfID == "" {
		return "", errors.New("missing shelf_id")
	}

	decoded, err := url.PathUnescape(shelfID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

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
		return "", errors.New("missing source_id")
	}

	decoded, err := url.PathUnescape(sourceID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

// readAssetName deliberately does not url.PathUnescape, unlike its neighbours
// here: ServeMux has already unescaped the wildcard, so decoding a second time
// rewrites any name that legitimately contains a percent escape. A file named
// "chart%20one.png" would be read as "chart one.png" and quietly serve a
// different file.
//
// The IDs above are shelf-generated and never contain '%', which is why the
// extra decode is unreachable for them; an asset name is chosen by whoever put
// the file on the shelf. Traversal is unaffected either way, because the name
// is validated after decoding, not before.
func readAssetName(r *http.Request) (string, error) {
	assetName := strings.TrimSpace(r.PathValue("asset_name"))
	if assetName == "" {
		return "", errors.New("missing asset_name")
	}

	return assetName, nil
}

func readLogID(r *http.Request) (string, error) {
	logID := strings.TrimSpace(r.PathValue("log_id"))
	if logID == "" {
		return "", errors.New("missing log_id")
	}

	decoded, err := url.PathUnescape(logID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
}

func readTaskChainID(r *http.Request) (string, error) {
	taskChainID := strings.TrimSpace(r.PathValue("taskchain_id"))
	if taskChainID == "" {
		return "", errors.New("missing taskchain_id")
	}

	decoded, err := url.PathUnescape(taskChainID)
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
