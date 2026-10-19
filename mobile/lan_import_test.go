package mobile

import (
	"encoding/json/v2"
	"testing"

	"github.com/voilelab/plainshelf/internal/lanshare"
)

// Rejection cases live in internal/lanshare's shared contract suite. This test
// checks that the gomobile bridge preserves both parsed fields and safe errors.
func TestParseLANShareQRBridge(t *testing.T) {
	text := `{"v":1,"url":"http://192.168.1.20/dav/","token":"private-token","shelves":[{"id":"one","name":"One","url":"http://192.168.1.20/dav/one/"}]}`
	out, err := ParseLANShareQR(text)
	if err != nil {
		t.Fatal(err)
	}
	var got lanshare.Payload
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Token != "private-token" || len(got.Shelves) != 1 || got.Shelves[0].Name != "One" {
		t.Fatal("bridge lost payload fields")
	}
	if out, err := ParseLANShareQR("{private-token"); err == nil || out != "" || err.Error() != "invalid QR JSON" {
		t.Fatal("bridge lost safe parser error")
	}
}
