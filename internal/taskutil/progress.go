package taskutil

import "sync"

// Progress is a thread-safe status and completion counter for Task
// implementations to embed.
//
// A task runs on the worker goroutine while API handlers read its status and
// percentage from request goroutines, so every field must be guarded.
type Progress struct {
	mu     sync.Mutex
	status Status
	done   int
	total  int
}

func (p *Progress) SetStatus(status Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status
}

// SetTotal records how many units of work the task expects to process.
func (p *Progress) SetTotal(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if total < 0 {
		total = 0
	}
	p.total = total
}

// Advance records one processed unit of work, whether it succeeded or not.
func (p *Progress) Advance() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done < p.total {
		p.done++
	}
}

func (p *Progress) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// Percentage returns processed units over total units, in the 0-100 range.
//
// Before the total is known the task has demonstrably made no progress, so it
// reports 0 — except once it has finished without failing, where having nothing
// left to process means it is complete.
func (p *Progress) Percentage() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.total <= 0 {
		if p.status.IsTerminal() && p.status != StatusFailed {
			return 100
		}
		return 0
	}
	return float64(p.done) / float64(p.total) * 100
}

func (p *Progress) Counts() (done, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done, p.total
}
