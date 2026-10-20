package jsonopt

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/voilelab/plainshelf/internal/util"
)

var ErrNotAnObject = util.NewError("JSON document is not an object")

type Object map[string]jsontext.Value

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

func (o Object) Set(name string, value any) error {
	raw, err := json.Marshal(value, Disk())
	if err != nil {
		return util.Errorf("member %q: %w", name, err)
	}
	o[name] = raw
	return nil
}

func (o Object) Delete(name string) {
	delete(o, name)
}

func (o Object) MarshalDisk() ([]byte, error) {
	data, err := json.Marshal(map[string]jsontext.Value(o), Disk())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return data, nil
}
