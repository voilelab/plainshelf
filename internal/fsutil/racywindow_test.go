package fsutil

import (
	"testing"
	"time"
)

func TestSettledRequiresMoreThanTheRacyWindow(t *testing.T) {
	readAt := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		modTime time.Time
		want    bool
	}{
		{"older than the window", readAt.Add(-RacyWindow - time.Nanosecond), true},
		{"exactly at the window", readAt.Add(-RacyWindow), false},
		{"inside the window", readAt.Add(-RacyWindow + time.Nanosecond), false},
		{"future timestamp", readAt.Add(time.Second), false},
		{"zero timestamp is older", time.Time{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Settled(tc.modTime, readAt); got != tc.want {
				t.Errorf("Settled(%v, %v) = %v, want %v", tc.modTime, readAt, got, tc.want)
			}
		})
	}
}
