// Package lanshareqr encodes the v1 sharing QR used by desktop and Android.
package lanshareqr

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/lanshare"
	"github.com/voilelab/plainshelf/internal/util"
	"rsc.io/qr"
)

// Encode returns a PNG data URL at the desktop's medium error correction level.
// Oversized payloads return the encoder's capacity error without a partial QR.
func Encode(payload lanshare.Payload) (string, error) {
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
