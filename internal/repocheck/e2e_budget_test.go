package repocheck

import (
	"path"
	"regexp"
	"strings"
	"testing"
)

const (
	e2eCaseLimit      = 40
	e2eSmokeCaseLimit = 20
	e2eSpecCaseLimit  = 5
)

var (
	playwrightCase = regexp.MustCompile(`(?m)^\s*test(?:\.(?:only|skip|fixme))?\(`)
	smokeTag       = regexp.MustCompile(`'@smoke'`)
)

func TestE2ECaseBudgets(t *testing.T) {
	root := repoRoot(t)
	total, smoke := 0, 0
	for _, file := range trackedFiles(t, root) {
		if path.Dir(file) != "e2e/tests" || !strings.HasSuffix(file, ".spec.ts") {
			continue
		}
		source := readFile(t, root, file)
		cases := len(playwrightCase.FindAll(source, -1))
		if cases > e2eSpecCaseLimit {
			t.Errorf("%s has %d E2E cases; the per-spec budget is %d", file, cases, e2eSpecCaseLimit)
		}
		total += cases
		smoke += len(smokeTag.FindAll(source, -1))
	}
	if total == 0 {
		t.Fatal("found no E2E cases; the case pattern no longer matches e2e/tests")
	}
	if total > e2eCaseLimit {
		t.Errorf("the E2E suite has %d cases; the budget is %d", total, e2eCaseLimit)
	}
	if smoke > e2eSmokeCaseLimit {
		t.Errorf("the @smoke tier has %d cases; the budget is %d", smoke, e2eSmokeCaseLimit)
	}
}
