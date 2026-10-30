package fsutil

import "time"

const RacyWindow = 2 * time.Second

func Settled(modTime, readAt time.Time) bool {
	return modTime.Before(readAt.Add(-RacyWindow))
}
