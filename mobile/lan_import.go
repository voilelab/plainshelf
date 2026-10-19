package mobile

import "github.com/voilelab/plainshelf/internal/lanshare"

// ParseLANShareQR uses the desktop reader's complete validation contract. The
// result is transient setup data: callers must never log or persist this JSON.
func ParseLANShareQR(text string) (string, error) {
	payload, err := lanshare.Parse(text)
	if err != nil {
		return "", err
	}
	return marshalJSON(payload)
}
