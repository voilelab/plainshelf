package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func resolveBookID(w http.ResponseWriter, r *http.Request) (string, bool) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return "", false
	}

	return bookID, true
}

func resolveSourceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return "", false
	}

	return sourceID, true
}

// resolveAssetName reads the asset name from the path. The name is only
// checked for presence here; shelf validates it as a safe file name before any
// filesystem access, and answers ErrInvalidAssetName for the rest.
func resolveAssetName(w http.ResponseWriter, r *http.Request) (string, bool) {
	assetName, err := readAssetName(r)
	if err != nil {
		http.Error(w, "invalid asset_name", http.StatusBadRequest)
		return "", false
	}

	return assetName, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}

	return true
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
