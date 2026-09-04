package bookpkg

import (
	"fmt"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrMalformedMetadata is the sentinel behind [MalformedMetadataError], so a
// caller that only needs to know "this file is not readable" — the API error
// table, a log filter — does not have to name the concrete type.
var ErrMalformedMetadata = util.NewError("metadata file could not be read as JSON")

// MalformedMetadataError reports a hand-editable metadata file this build could
// not decode: book.json, a source's meta.json, trash.json, shelf.json.
//
// It exists to name the file. encoding/json/v2 says precisely what is wrong and
// which member is at fault — `duplicate object member name "title"`, `invalid
// UTF-8 within "/comments"` — but says it about an anonymous byte stream, which
// on a shelf holding a thousand book.json files is not something a user can act
// on. Wrapping the decoder's own message keeps the member name and adds the
// path.
//
// It deliberately does not cover the caches under app/. Those are rebuildable,
// so an unreadable one is discarded and recomputed rather than reported; see
// shelf/scancache and shelf/fingerprint.
type MalformedMetadataError struct {
	// File is the shelf-relative path of the file that could not be read.
	File string
	// Err is the decoder's own error, which names the offending member.
	Err error
}

func (e *MalformedMetadataError) Error() string {
	return fmt.Sprintf("%s: %v", e.File, e.Err)
}

// Unwrap reports both the sentinel and the decoder's error, so errors.Is
// matches ErrMalformedMetadata and the underlying jsontext or json error alike.
func (e *MalformedMetadataError) Unwrap() []error {
	return []error{ErrMalformedMetadata, e.Err}
}

// malformedMetadata wraps a decode failure on file. It returns nil for a nil
// error so a call site can wrap unconditionally.
func malformedMetadata(file string, err error) error {
	if err == nil {
		return nil
	}
	return &MalformedMetadataError{File: file, Err: err}
}
