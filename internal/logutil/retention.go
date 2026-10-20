package logutil

import "sync/atomic"

const retentionUnset = -1

type Retention struct {
	days atomic.Int64
}

func NewRetention() *Retention {
	r := &Retention{}
	r.Clear()
	return r
}

func (r *Retention) Set(days int) {
	if r == nil {
		return
	}
	r.days.Store(int64(max(days, 0)))
}

func (r *Retention) Clear() {
	if r == nil {
		return
	}
	r.days.Store(retentionUnset)
}

func (r *Retention) Days(configured int) int {
	if r == nil {
		return configured
	}
	if days := r.days.Load(); days != retentionUnset {
		return int(days)
	}
	return configured
}
