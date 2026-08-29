package logutil

import "sync/atomic"

// retentionUnset marks a Retention carrying no runtime value, which leaves each
// writer on the window its own configuration names.
const retentionUnset = -1

// Retention is a retention window shared by every rotating writer an app
// builds, so a change made while the server runs reaches all of them without a
// restart. A nil *Retention is usable and simply never overrides anything.
//
// It holds a value rather than calling back into whoever owns the setting
// because a writer reads it during rotation, while it holds the lock the logger
// itself needs: a callback that logged — a store read reporting an error, say —
// would deadlock the logger it is part of.
type Retention struct {
	days atomic.Int64
}

// NewRetention returns a window that overrides nothing yet.
func NewRetention() *Retention {
	r := &Retention{}
	r.Clear()
	return r
}

// Set overrides the configured window for every writer sharing this value.
// Zero keeps every file.
func (r *Retention) Set(days int) {
	if r == nil {
		return
	}
	if days < 0 {
		days = 0
	}
	r.days.Store(int64(days))
}

// Clear returns every writer sharing this value to its own configured window.
func (r *Retention) Clear() {
	if r == nil {
		return
	}
	r.days.Store(retentionUnset)
}

// Days reports the window to apply to a writer configured with configured.
func (r *Retention) Days(configured int) int {
	if r == nil {
		return configured
	}
	if days := r.days.Load(); days != retentionUnset {
		return int(days)
	}
	return configured
}
