package server

import (
	"testing"
	"time"
)

func TestCalendarDateUsesTheLocalDate(t *testing.T) {
	locations := []*time.Location{
		time.FixedZone("UTC+8", 8*60*60),
		time.FixedZone("UTC-7", -7*60*60),
	}

	for _, location := range locations {
		t.Run(location.String(), func(t *testing.T) {
			now := time.Date(2026, time.July, 20, 2, 30, 0, 0, location)
			got := calendarDate(now)
			want := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
			if got != want {
				t.Fatalf("calendarDate(%v) = %v, want %v", now, got, want)
			}
		})
	}
}

func TestReadingActivityDateAllowsLocalYesterday(t *testing.T) {
	now := time.Date(2026, time.July, 20, 2, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	got, err := readingActivityDate("2026-07-19", now)
	if err != nil {
		t.Fatalf("readingActivityDate: %v", err)
	}
	want := time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC)
	if got != want {
		t.Fatalf("readingActivityDate(yesterday) = %v, want %v", got, want)
	}
}

func TestReadingActivityDateClampsOutOfWindowDate(t *testing.T) {
	now := time.Date(2026, time.July, 20, 2, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	got, err := readingActivityDate("2026-07-18", now)
	if err != nil {
		t.Fatalf("readingActivityDate: %v", err)
	}
	want := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	if got != want {
		t.Fatalf("readingActivityDate(out of window) = %v, want %v", got, want)
	}
}
