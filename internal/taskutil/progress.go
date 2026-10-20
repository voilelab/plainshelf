package taskutil

import "sync"

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

func (p *Progress) SetTotal(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total = max(total, 0)
}

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
