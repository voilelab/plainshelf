package util

import (
	"strings"
	"time"
)

// JSONDate is a date-only value (no time-of-day or timezone), marshaled as
// "YYYY-MM-DD". UnmarshalJSON also accepts legacy RFC3339 input (previously
// produced by JSONTime) and keeps only its date component, taken in the
// timestamp's own offset rather than converting to UTC first. This lets
// existing book.json files that still hold a full timestamp keep loading
// correctly; the next write persists them back in date-only form (lazy
// migration, no one-shot migration tool needed).
type JSONDate time.Time

func (d JSONDate) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(d).Format(time.DateOnly) + `"`), nil
}

func (d *JSONDate) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if parsed, err := time.Parse(time.DateOnly, s); err == nil {
		*d = JSONDate(parsed)
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return Errorf("%w", err)
	}
	y, m, day := parsed.Date()
	*d = JSONDate(time.Date(y, m, day, 0, 0, 0, 0, time.UTC))
	return nil
}

func (d JSONDate) IsZero() bool {
	return time.Time(d).IsZero()
}
