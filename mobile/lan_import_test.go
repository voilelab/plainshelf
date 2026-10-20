package mobile

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/lanshare"
)

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
	if out, err := ParseLANShareQR("{private-token"); err == nil || out != "" || !strings.Contains(err.Error(), "invalid QR JSON") || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("bridge lost safe parser error: output %q, error %v", out, err)
	}
}
