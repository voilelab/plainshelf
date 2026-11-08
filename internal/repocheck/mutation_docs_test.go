package repocheck

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
)

func TestMutationDocNamesStrykerScope(t *testing.T) {
	root := repoRoot(t)
	var conf struct {
		Mutate []string `json:"mutate"`
	}
	if err := json.Unmarshal(readFile(t, root, "frontend/stryker.conf.json"), &conf); err != nil {
		t.Fatalf("parse stryker.conf.json: %v", err)
	}
	doc := string(readFile(t, root, "docs/development/mutation-testing.md"))
	for _, pattern := range conf.Mutate {
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		want := "`" + path.Base(pattern) + "`"
		if strings.ContainsAny(pattern, "*?") {
			want = "`frontend/" + path.Dir(pattern) + "/`"
		}
		if !strings.Contains(doc, want) {
			t.Errorf("stryker.conf.json mutates %s, but mutation-testing.md never names %s", pattern, want)
		}
	}
}
