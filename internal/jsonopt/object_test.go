package jsonopt_test

import (
	"errors"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

func TestObjectRoundTripKeepsUnknownMembers(t *testing.T) {
	input := `{"zeta":{"b":[1,{"x":null}],"a":"keep"},"icon":"book","alpha":true}`

	obj, err := jsonopt.ParseObject([]byte(input))
	if err != nil {
		t.Fatalf("ParseObject: %v", err)
	}
	if err := obj.Set("icon", "star"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := obj.MarshalDisk()
	if err != nil {
		t.Fatalf("MarshalDisk: %v", err)
	}

	want := `{
  "alpha": true,
  "icon": "star",
  "zeta": {
    "b": [
      1,
      {
        "x": null
      }
    ],
    "a": "keep"
  }
}`
	if string(got) != want {
		t.Fatalf("MarshalDisk:\n%s\nwant:\n%s", got, want)
	}
}

func TestObjectMarshalIsDeterministic(t *testing.T) {
	obj, err := jsonopt.ParseObject([]byte(`{"zeta":1,"alpha":2,"mu":3,"beta":4,"omega":5,"delta":6}`))
	if err != nil {
		t.Fatalf("ParseObject: %v", err)
	}
	first, err := obj.MarshalDisk()
	if err != nil {
		t.Fatalf("MarshalDisk: %v", err)
	}
	for range marshalRounds {
		again, err := obj.MarshalDisk()
		if err != nil {
			t.Fatalf("MarshalDisk: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("MarshalDisk is not deterministic:\n%s\n%s", first, again)
		}
	}
}

func TestObjectLookupAndDelete(t *testing.T) {
	obj, err := jsonopt.ParseObject([]byte(`{"n":1,"s":"x"}`))
	if err != nil {
		t.Fatalf("ParseObject: %v", err)
	}

	var n int
	if found, err := obj.Lookup("n", &n); !found || err != nil || n != 1 {
		t.Fatalf("Lookup(n) = %v, %v, %d", found, err, n)
	}
	var s string
	if found, err := obj.Lookup("missing", &s); found || err != nil {
		t.Fatalf("Lookup(missing) = %v, %v", found, err)
	}
	if found, err := obj.Lookup("s", &n); !found || err == nil {
		t.Fatalf("Lookup(s) into an int = %v, %v; want a type error", found, err)
	}

	obj.Delete("n")
	obj.Delete("missing")
	if _, ok := obj["n"]; ok {
		t.Fatal("Delete left the member in place")
	}
}

func TestParseObjectRefusals(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		notObj bool
	}{
		{name: "array", input: `[1,2]`, notObj: true},
		{name: "string", input: `"x"`, notObj: true},
		{name: "null", input: `null`, notObj: true},
		{name: "truncated", input: `{"a":`},
		{name: "duplicate member", input: `{"a":1,"a":2}`},
		{name: "nested duplicate", input: `{"a":{"b":1,"b":2}}`},
		{name: "invalid UTF-8", input: "{\"a\":\"\xff\"}"},
		{name: "trailing data", input: `{} {}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jsonopt.ParseObject([]byte(tc.input))
			if err == nil {
				t.Fatal("ParseObject accepted it")
			}
			if got := errors.Is(err, jsonopt.ErrNotAnObject); got != tc.notObj {
				t.Fatalf("errors.Is(ErrNotAnObject) = %v, want %v (err: %v)", got, tc.notObj, err)
			}
		})
	}
}
