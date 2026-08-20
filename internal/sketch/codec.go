package sketch

import (
	"encoding/base64"
	"encoding/binary"

	"github.com/voilelab/plainshelf/internal/util"
)

// The encoded form is a version byte, the shingle width, the exact distinct
// shingle count, the number of retained hashes, and then the hashes themselves,
// all little-endian.
const (
	codecVersion = 1
	headerLen    = 17
)

// Encode serializes the sketch as base64, for storing it next to the document
// it fingerprints.
func (s Sketch) Encode() string {
	raw := make([]byte, headerLen, headerLen+len(s.Values)*8)
	raw[0] = codecVersion
	binary.LittleEndian.PutUint32(raw[1:5], uint32(s.N))
	binary.LittleEndian.PutUint64(raw[5:13], uint64(s.Distinct))
	binary.LittleEndian.PutUint32(raw[13:17], uint32(len(s.Values)))

	for _, value := range s.Values {
		raw = binary.LittleEndian.AppendUint64(raw, value)
	}

	return base64.StdEncoding.EncodeToString(raw)
}

// Decode parses a sketch produced by Encode, rejecting anything that would not
// compare correctly rather than returning a silently wrong sketch.
func Decode(encoded string) (Sketch, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Sketch{}, util.Errorf("base64: %w", err)
	}
	if len(raw) < headerLen {
		return Sketch{}, util.Errorf("truncated sketch: %d bytes", len(raw))
	}
	if raw[0] != codecVersion {
		return Sketch{}, util.Errorf("unsupported sketch version %d", raw[0])
	}

	n := binary.LittleEndian.Uint32(raw[1:5])
	distinct := binary.LittleEndian.Uint64(raw[5:13])
	count := binary.LittleEndian.Uint32(raw[13:17])

	if n < 1 {
		return Sketch{}, util.NewError("sketch shingle width is zero")
	}
	if uint64(count) > distinct {
		return Sketch{}, util.Errorf("sketch holds %d hashes but counts %d distinct shingles", count, distinct)
	}
	if uint64(len(raw)) != uint64(headerLen)+uint64(count)*8 {
		return Sketch{}, util.Errorf("sketch holds %d hashes but has %d bytes", count, len(raw))
	}

	values := make([]uint64, count)
	for i := range values {
		offset := headerLen + i*8
		values[i] = binary.LittleEndian.Uint64(raw[offset : offset+8])
		if i > 0 && values[i] <= values[i-1] {
			return Sketch{}, util.Errorf("sketch hashes are not strictly ascending at index %d", i)
		}
	}

	return Sketch{N: int(n), Distinct: int(distinct), Values: values}, nil
}
