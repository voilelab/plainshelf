package taskutil

import "testing"

func chainOf(statuses ...Status) *TaskChain {
	tasks := make([]Task, 0, len(statuses))
	for _, status := range statuses {
		tasks = append(tasks, &statusTask{status: status})
	}
	return &TaskChain{Tasks: tasks}
}

func TestTaskChainStatusAggregation(t *testing.T) {
	cases := []struct {
		name  string
		chain *TaskChain
		want  Status
	}{
		{"empty chain has nothing left to do", chainOf(), StatusCompleted},
		{"untouched", chainOf(StatusPending, StatusPending), StatusPending},
		{"first task started", chainOf(StatusRunning, StatusPending), StatusRunning},
		{"partway through", chainOf(StatusCompleted, StatusPending), StatusRunning},
		{"all done", chainOf(StatusCompleted, StatusCompleted), StatusCompleted},
		{"failure dominates", chainOf(StatusCompleted, StatusFailed), StatusFailed},
		{"failure dominates a partial result", chainOf(StatusPartiallyCompleted, StatusFailed), StatusFailed},
		{"partial result surfaces", chainOf(StatusPartiallyCompleted, StatusCompleted), StatusPartiallyCompleted},
		{"partial result while still running", chainOf(StatusPartiallyCompleted, StatusRunning), StatusPartiallyCompleted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.chain.Status(); got != tc.want {
				t.Errorf("Status() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskChainPercentageAveragesTasks(t *testing.T) {
	chain := &TaskChain{Tasks: []Task{
		&statusTask{percentage: 100},
		&statusTask{percentage: 50},
	}}

	if got := chain.Percentage(); got != 75 {
		t.Errorf("Percentage() = %v, want 75", got)
	}
}

func TestTaskChainPercentageClampsTaskValues(t *testing.T) {
	chain := &TaskChain{Tasks: []Task{
		&statusTask{percentage: 400},
		&statusTask{percentage: -100},
	}}

	if got := chain.Percentage(); got != 50 {
		t.Errorf("Percentage() = %v, want 50", got)
	}
}

func TestTaskChainPercentageOfEmptyChain(t *testing.T) {
	if got := (&TaskChain{}).Percentage(); got != 100 {
		t.Errorf("Percentage() = %v, want 100", got)
	}
}

func TestStatusString(t *testing.T) {
	cases := map[Status]string{
		StatusPending:            "pending",
		StatusRunning:            "running",
		StatusPartiallyCompleted: "partially_completed",
		StatusCompleted:          "completed",
		StatusFailed:             "failed",
		Status(99):               "unknown",
	}

	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", status, got, want)
		}
	}
}

func TestStatusIsTerminal(t *testing.T) {
	terminal := []Status{StatusPartiallyCompleted, StatusCompleted, StatusFailed}
	for _, status := range terminal {
		if !status.IsTerminal() {
			t.Errorf("Expected %v to be terminal", status)
		}
	}

	for _, status := range []Status{StatusPending, StatusRunning} {
		if status.IsTerminal() {
			t.Errorf("Expected %v to be non-terminal", status)
		}
	}
}
