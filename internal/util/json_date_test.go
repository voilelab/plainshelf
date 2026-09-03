package util

import (
	"encoding/json/v2"
	"testing"
	"time"
)

func TestJSONDateMarshalDateOnly(t *testing.T) {
	d := JSONDate(time.Date(2026, time.March, 15, 8, 30, 0, 0, time.UTC))
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Failed to marshal JSONDate: %v", err)
	}
	if got, want := string(data), `"2026-03-15"`; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
}

func TestJSONDateUnmarshalDateOnly(t *testing.T) {
	var d JSONDate
	if err := json.Unmarshal([]byte(`"2026-03-15"`), &d); err != nil {
		t.Fatalf("Failed to unmarshal date-only: %v", err)
	}
	y, m, day := time.Time(d).Date()
	if y != 2026 || m != time.March || day != 15 {
		t.Errorf("Expected 2026-03-15, got %04d-%02d-%02d", y, m, day)
	}
}

func TestJSONDateUnmarshalRFC3339Compat(t *testing.T) {
	var d JSONDate
	if err := json.Unmarshal([]byte(`"2026-03-15T08:30:00Z"`), &d); err != nil {
		t.Fatalf("Failed to unmarshal RFC3339: %v", err)
	}
	y, m, day := time.Time(d).Date()
	if y != 2026 || m != time.March || day != 15 {
		t.Errorf("Expected 2026-03-15, got %04d-%02d-%02d", y, m, day)
	}
}

// The date must be taken in the timestamp's own offset, not after converting
// to UTC. "2026-03-16T00:30:00+09:00" is 2026-03-15T15:30:00Z, so a UTC-first
// reading would wrongly yield 2026-03-15. Lock in 2026-03-16.
func TestJSONDateUnmarshalRFC3339OffsetBoundary(t *testing.T) {
	var d JSONDate
	if err := json.Unmarshal([]byte(`"2026-03-16T00:30:00+09:00"`), &d); err != nil {
		t.Fatalf("Failed to unmarshal RFC3339 with offset: %v", err)
	}
	y, m, day := time.Time(d).Date()
	if y != 2026 || m != time.March || day != 16 {
		t.Errorf("Expected 2026-03-16, got %04d-%02d-%02d", y, m, day)
	}
}

func TestJSONDateRoundTrip(t *testing.T) {
	var d JSONDate
	if err := json.Unmarshal([]byte(`"2026-03-15"`), &d); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	if got, want := string(data), `"2026-03-15"`; got != want {
		t.Errorf("Round trip changed value: expected %s, got %s", want, got)
	}
}

func TestJSONDateIsZero(t *testing.T) {
	var zero JSONDate
	if !zero.IsZero() {
		t.Error("Expected zero JSONDate to report IsZero() == true")
	}
	nonZero := JSONDate(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC))
	if nonZero.IsZero() {
		t.Error("Expected non-zero JSONDate to report IsZero() == false")
	}
}

func TestJSONDateUnmarshalInvalid(t *testing.T) {
	var d JSONDate
	if err := json.Unmarshal([]byte(`"not-a-date"`), &d); err == nil {
		t.Error("Expected error unmarshaling invalid date string, got nil")
	}
}

func TestJSONDateOmitzeroInStruct(t *testing.T) {
	type wrapper struct {
		X JSONDate `json:"x,omitzero"`
	}

	data, err := json.Marshal(wrapper{})
	if err != nil {
		t.Fatalf("Failed to marshal zero wrapper: %v", err)
	}
	if got, want := string(data), `{}`; got != want {
		t.Errorf("Expected zero value to omit key, got %s", got)
	}

	data, err = json.Marshal(wrapper{X: JSONDate(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC))})
	if err != nil {
		t.Fatalf("Failed to marshal non-zero wrapper: %v", err)
	}
	if got, want := string(data), `{"x":"2026-03-15"}`; got != want {
		t.Errorf("Expected non-zero value to emit key, got %s", got)
	}
}
