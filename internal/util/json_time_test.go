package util

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJSONTime(t *testing.T) {
	now := time.Now().Truncate(time.Second) // truncate to second for comparison

	jsonTime := JSONTime(now)

	data, err := json.Marshal(jsonTime)
	if err != nil {
		t.Fatalf("Failed to marshal JSONTime: %v", err)
	}

	var unmarshaledTime JSONTime
	err = json.Unmarshal(data, &unmarshaledTime)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSONTime: %v", err)
	}

	if time.Time(unmarshaledTime) != now {
		t.Errorf("Expected %v, got %v", now, time.Time(unmarshaledTime))
	}
}
