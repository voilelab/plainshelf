package bookpkg

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/voilelab/plainshelf/internal/util"
)

var ErrMalformedMetadata = util.NewError("metadata file could not be read as JSON")

type MalformedMetadataError struct {
	File string
	Err  error
}

func (e *MalformedMetadataError) Error() string {
	return fmt.Sprintf("%s: %v", e.File, e.Err)
}

func (e *MalformedMetadataError) Unwrap() []error {
	return []error{ErrMalformedMetadata, e.Err}
}

func NameMetadataDecodeError(file string, err error) error {
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
