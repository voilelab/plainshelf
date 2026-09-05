package bookpkg

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrMalformedMetadata is the sentinel behind [MalformedMetadataError], so a
// caller that only needs "this file is not readable" need not name the type.
var ErrMalformedMetadata = util.NewError("metadata file could not be read as JSON")

// MalformedMetadataError reports a hand-editable metadata file this build could
// not decode: book.json, a source's meta.json, trash.json, shelf.json.
//
// It exists to name the file: encoding/json/v2 says which member is at fault
// but says it about an anonymous byte stream, which on a shelf holding a
// thousand book.json files is not something a user can act on.
//
// It deliberately does not cover the caches under app/. Those are rebuildable,
// so an unreadable one is discarded and recomputed rather than reported; see
// shelf/scancache and shelf/fingerprint.
type MalformedMetadataError struct {
	// File is the shelf-relative path of the file that could not be read.
	File string
	Err  error
}

func (e *MalformedMetadataError) Error() string {
	return fmt.Sprintf("%s: %v", e.File, e.Err)
}

// Unwrap reports both, so errors.Is matches ErrMalformedMetadata and the
// underlying jsontext or json error alike.
func (e *MalformedMetadataError) Unwrap() []error {
	return []error{ErrMalformedMetadata, e.Err}
}

// MetadataReadError attributes a failed read of one of those files, and returns
// nil for a nil error so a call site can wrap unconditionally.
//
// Only a decoder error becomes a [MalformedMetadataError]. json.UnmarshalRead
// reports a failure of the reader underneath it the same way it reports a
// syntax error, and a read that died mid-file on a disconnected SMB share is
// not a file anyone can repair.
func MetadataReadError(file string, err error) error {
	if err == nil {
		return nil
	}

	var syntactic *jsontext.SyntacticError
	var semantic *json.SemanticError
	if errors.As(err, &syntactic) || errors.As(err, &semantic) {
		return &MalformedMetadataError{File: file, Err: err}
	}
	return err
}
