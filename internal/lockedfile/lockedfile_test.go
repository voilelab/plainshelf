package lockedfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	childPathEnv = "LOCKEDFILE_CHILD_PATH"
	childWrites  = 40
)

func parseCount(text string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(text))
	return n
}

func formatCount(n int) (string, error) { return strconv.Itoa(n), nil }

func TestProcessesShareTheLock(t *testing.T) {
	if path := os.Getenv(childPathEnv); path != "" {
		file := New(path, "counter", ".counter-*.txt")
		for range childWrites {
			if _, err := Mutate(file, parseCount, formatCount, func(n int) int { return n + 1 }); err != nil {
				t.Fatalf("child mutate: %v", err)
			}
		}
		return
	}

	path := filepath.Join(t.TempDir(), "counter.txt")
	const children = 2
	var wg sync.WaitGroup
	for i := range children {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestProcessesShareTheLock$")
			cmd.Env = append(os.Environ(), childPathEnv+"="+path)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("child %d: %v\n%s", i, err, out)
			}
		})
	}
	wg.Wait()
	if t.Failed() {
		return
	}

	raw, err := New(path, "counter", ".counter-*.txt").Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := parseCount(raw); got != children*childWrites {
		t.Fatalf("counter = %d, want %d", got, children*childWrites)
	}
}

func TestFileNeedsPath(t *testing.T) {
	file := New("", "counter", ".counter-*.txt")
	if _, err := file.Read(); err == nil {
		t.Fatal("read succeeded without a path")
	}
	if _, err := Mutate(file, parseCount, formatCount, func(n int) int { return n + 1 }); err == nil {
		t.Fatal("mutate succeeded without a path")
	}
}

func TestMutateIfSkipsUnneededWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "counter.txt")
	if err := os.WriteFile(path, []byte(" 3 "), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	file := New(path, "counter", ".counter-*.txt")
	increment := func(n int) int { return n + 1 }

	got, err := MutateIf(file, parseCount, formatCount, func(n int) bool { return n > 5 }, increment)
	if err != nil || got != " 3 " {
		t.Fatalf("unneeded MutateIf = %q, %v; want raw text unchanged", got, err)
	}

	got, err = MutateIf(file, parseCount, formatCount, func(n int) bool { return n == 3 }, increment)
	if err != nil || got != "4" {
		t.Fatalf("needed MutateIf = %q, %v; want serialized result", got, err)
	}
	raw, err := file.Read()
	if err != nil || raw != "4" {
		t.Fatalf("file after MutateIf = %q, %v; want written result", raw, err)
	}
}
