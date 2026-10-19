// Package lanshareqr encodes the v1 sharing QR used by desktop and Android.
package lanshareqr

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
	"rsc.io/qr"
)

// Version is the sharing payload version understood by readers.
const Version = 1

// Payload is the connection information encoded in a sharing QR.
type Payload struct {
	V     int    `json:"v"`
	URL   string `json:"url"`
	Token string `json:"token"`
	// Shelves lists only the ticked shelves, so the code changes with the
	// selection.
	Shelves []Shelf `json:"shelves"`
}

// Shelf names one selected shelf and its WebDAV URL.
type Shelf struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Encode returns a PNG data URL at the desktop's medium error correction level.
// Oversized payloads return the encoder's capacity error without a partial QR.
func Encode(payload Payload) (string, error) {
	text, err := json.Marshal(payload, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	code, err := qr.Encode(string(text), qr.M)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("data:image/png;base64,")
	buf.WriteString(base64.StdEncoding.EncodeToString(code.PNG()))
	return buf.String(), nil
}
