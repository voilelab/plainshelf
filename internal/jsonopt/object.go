package jsonopt

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrNotAnObject is returned by ParseObject for a document whose top-level
// value is not a JSON object, null included.
var ErrNotAnObject = util.NewError("JSON document is not an object")

// Object is a JSON object held member by member, for a writer that must keep
// the members it does not understand: one a newer build added, or one a user
// wrote by hand.
//
// Each member keeps its value as written, nested structure included. What is
// not kept is formatting: [Object.MarshalDisk] re-indents every value and sorts
// the top-level names, so a round trip is equal in content but not in bytes.
//
// folder.json is the first file written this way; book.json and shelf.json are
// meant to move onto it rather than grow a second mechanism.
type Object map[string]jsontext.Value

// ParseObject decodes data as one JSON object. It applies the v2 defaults every
// PlainShelf reader uses, so a duplicate member name or invalid UTF-8 anywhere
// in the document is an error rather than a member silently chosen or mangled.
func ParseObject(data []byte) (Object, error) {
	var obj Object
	if err := json.Unmarshal(data, &obj); err != nil {
		if jsontext.Value(data).Kind() != '{' {
			return nil, util.Errorf("%w: %w", ErrNotAnObject, err)
		}
		return nil, util.Errorf("%w", err)
	}
	if obj == nil {
		return nil, util.Errorf("%w", ErrNotAnObject)
	}
	return obj, nil
}

// Lookup decodes the named member into target. It reports false, leaving target
// untouched, when the member is absent.
func (o Object) Lookup(name string, target any) (bool, error) {
	raw, ok := o[name]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return true, util.Errorf("member %q: %w", name, err)
	}
	return true, nil
}

// Set replaces the named member with value, encoded deterministically.
func (o Object) Set(name string, value any) error {
	raw, err := json.Marshal(value, Disk())
	if err != nil {
		return util.Errorf("member %q: %w", name, err)
	}
	o[name] = raw
	return nil
}

// Delete removes the named member; deleting an absent one is a no-op.
func (o Object) Delete(name string) {
	delete(o, name)
}

// MarshalDisk encodes the object with the [Disk] options.
func (o Object) MarshalDisk() ([]byte, error) {
	data, err := json.Marshal(map[string]jsontext.Value(o), Disk())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return data, nil
}
