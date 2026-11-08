package repocheck

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type mkdocsConfig struct {
	Nav         []any  `yaml:"nav"`
	ExcludeDocs string `yaml:"exclude_docs"`
}

func TestDocsNavListsEveryPage(t *testing.T) {
	root := repoRoot(t)
	var conf mkdocsConfig
	if err := yaml.Unmarshal(readFile(t, root, "mkdocs.yml"), &conf); err != nil {
		t.Fatalf("parse mkdocs.yml: %v", err)
	}
	excluded := excludedDocPrefixes(t, conf.ExcludeDocs)

	var pages []string
	for _, file := range trackedFiles(t, root) {
		page, ok := strings.CutPrefix(file, "docs/")
		if !ok || !strings.HasSuffix(page, ".md") {
			continue
		}
		if !slices.ContainsFunc(excluded, func(prefix string) bool { return strings.HasPrefix(page, prefix) }) {
			pages = append(pages, page)
		}
	}

	nav := navPages(conf.Nav)
	for _, entry := range nav {
		if !slices.Contains(pages, entry) {
			t.Errorf("mkdocs.yml nav lists %s, which is not a published page under docs/", entry)
		}
	}
	for _, page := range pages {
		if !slices.Contains(nav, page) {
			t.Errorf("docs/%s is not in the mkdocs.yml nav; add it or list it under exclude_docs", page)
		}
	}
}

func excludedDocPrefixes(t *testing.T, excludeDocs string) []string {
	t.Helper()
	var prefixes []string
	for _, line := range strings.Split(excludeDocs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		prefix, ok := strings.CutPrefix(line, "/")
		if !ok || !strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, "*?![") {
			t.Fatalf("exclude_docs pattern %q is not an anchored directory; teach this test the pattern", line)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func navPages(node any) []string {
	switch v := node.(type) {
	case string:
		if strings.HasSuffix(v, ".md") {
			return []string{v}
		}
	case []any:
		var pages []string
		for _, item := range v {
			pages = append(pages, navPages(item)...)
		}
		return pages
	case map[string]any:
		var pages []string
		for _, item := range v {
			pages = append(pages, navPages(item)...)
		}
		return pages
	}
	return nil
}
