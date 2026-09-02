package server

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/voilelab/plainshelf/internal/util"
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

var jsonRequestOptions = jsonv2.JoinOptions(
	jsonv2.RejectUnknownMembers(true),
)

func decodeRequestJSON(body io.Reader, v any, optional bool) error {
	decoder := jsontext.NewDecoder(body)

	if decoder.PeekKind() == jsontext.KindInvalid {
		// jsonv2.UnmarshalRead reports an empty body and a truncated one alike
		// as "unexpected EOF"; only reading the first token separates them.
		_, err := decoder.ReadToken()
		if err == nil {
			return util.Errorf("no JSON value in the request body")
		}
		if optional && errors.Is(err, io.EOF) {
			return nil
		}
		return util.Errorf("%w", err)
	}

	if err := jsonv2.UnmarshalDecode(decoder, v, jsonRequestOptions); err != nil {
		return util.Errorf("%w", err)
	}

	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		if err == nil {
			return util.Errorf("unexpected data after the top-level JSON value")
		}
		return util.Errorf("%w", err)
	}
	return nil
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONBody(w, r, v, false)
}

// decodeOptionalStrictJSON is decodeStrictJSON that treats an empty body as "no
// fields set", leaving v at its zero value. It suits a request whose every field
// is optional - copying a book carries only an optional destination folder, so an
// empty body is a valid "copy in place" rather than a malformed request.
//
// It streams through jsontext.Decoder rather than buffering the body, so an
// oversized or whitespace-only payload cannot force an unbounded read before
// validation. An empty body surfaces as io.EOF on the first token.
func decodeOptionalStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONBody(w, r, v, true)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any, optional bool) bool {
	err := decodeRequestJSON(r.Body, v, optional)
	if err == nil {
		return true
	}
	if isRequestBodyTooLarge(err) {
		http.Error(w, fmt.Sprintf("request body too large (max %d MB)", maxImportBodySize>>20), http.StatusRequestEntityTooLarge)
		return false
	}
	http.Error(w, jsonDecodeMessage(err), http.StatusBadRequest)
	return false
}

// jsonDecodeMessage names the field the decoder stopped on, when it reports
// one. The decoder's own text is not used: it names Go types, which the client
// has no business seeing.
func jsonDecodeMessage(err error) string {
	const prefix = "invalid JSON"

	var pointer jsontext.Pointer
	if semantic, ok := errors.AsType[*jsonv2.SemanticError](err); ok {
		pointer = semantic.JSONPointer
	} else if syntactic, ok := errors.AsType[*jsontext.SyntacticError](err); ok {
		pointer = syntactic.JSONPointer
	}

	field, ok := describableField(pointer)
	if !ok {
		return prefix
	}
	return prefix + " at " + strconv.Quote(field)
}

const maxDescribedFieldLength = 64

// describableField renders a JSON pointer as a slash-separated field path, and
// reports whether it is safe to show. A member name is the caller's own text,
// so anything that does not read as a field name is withheld — belt-and-braces
// alongside the nosniff header the server now sends, not a substitute for it.
func describableField(pointer jsontext.Pointer) (string, bool) {
	var path strings.Builder
	for token := range pointer.Tokens() {
		if path.Len() > 0 {
			path.WriteByte('/')
		}
		path.WriteString(token)
	}

	field := path.String()
	if field == "" || len(field) > maxDescribedFieldLength {
		return "", false
	}
	for _, r := range field {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_-./", r) {
			return "", false
		}
	}
	return field, true
}

func isRequestBodyTooLarge(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}
