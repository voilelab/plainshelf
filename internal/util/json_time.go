package util

import "time"

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format(time.RFC3339) + `"`), nil
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	parsedTime, err := time.Parse(`"`+time.RFC3339+`"`, string(data))
	if err != nil {
		return Errorf("%w", err)
	}
	*t = JSONTime(parsedTime)
	return nil
}

func (t JSONTime) IsZero() bool {
	return time.Time(t).IsZero()
}
