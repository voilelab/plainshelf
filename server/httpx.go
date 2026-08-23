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

// decodeOptionalStrictJSON is decodeStrictJSON that treats an empty body as "no
// fields set", leaving v at its zero value. It suits a request whose every field
// is optional - copying a book carries only an optional destination folder, so an
// empty body is a valid "copy in place" rather than a malformed request.
//
// It streams through json.Decoder rather than buffering the body, so an
// oversized or whitespace-only payload cannot force an unbounded read before
// validation. An empty body surfaces as io.EOF on the first token.
func decodeOptionalStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
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
