package taskutil

import "testing"

func TestProgressPercentageIsZeroBeforeTotalIsKnown(t *testing.T) {
	var p Progress
	p.SetStatus(StatusRunning)

	if got := p.Percentage(); got != 0 {
		t.Errorf("Expected 0%% before the total is known, got %v", got)
	}
}

func TestProgressPercentageTracksProcessedUnits(t *testing.T) {
	var p Progress
	p.SetTotal(4)

	if got := p.Percentage(); got != 0 {
		t.Errorf("Expected 0%%, got %v", got)
	}

	p.Advance()
	if got := p.Percentage(); got != 25 {
		t.Errorf("Expected 25%%, got %v", got)
	}

	p.Advance()
	p.Advance()
	p.Advance()
	if got := p.Percentage(); got != 100 {
		t.Errorf("Expected 100%%, got %v", got)
	}
}

func TestProgressAdvanceStopsAtTotal(t *testing.T) {
	var p Progress
	p.SetTotal(1)

	p.Advance()
	p.Advance()

	done, total := p.Counts()
	if done != 1 || total != 1 {
		t.Errorf("Expected 1/1, got %d/%d", done, total)
	}
	if got := p.Percentage(); got != 100 {
		t.Errorf("Expected 100%%, got %v", got)
	}
}

func TestProgressEmptyWorkloadIsCompleteOnlyWhenFinished(t *testing.T) {
	var p Progress
	p.SetTotal(0)
	p.SetStatus(StatusRunning)

	if got := p.Percentage(); got != 0 {
		t.Errorf("Expected 0%% while still running, got %v", got)
	}

	p.SetStatus(StatusCompleted)
	if got := p.Percentage(); got != 100 {
		t.Errorf("Expected 100%% once complete with nothing to process, got %v", got)
	}

	p.SetStatus(StatusFailed)
	if got := p.Percentage(); got != 0 {
		t.Errorf("Expected 0%% for a failed task that processed nothing, got %v", got)
	}
}

func TestProgressSetTotalRejectsNegative(t *testing.T) {
	var p Progress
	p.SetTotal(-5)

	if _, total := p.Counts(); total != 0 {
		t.Errorf("Expected a negative total to clamp to 0, got %d", total)
	}
}
