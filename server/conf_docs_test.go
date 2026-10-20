package server

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/httputil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
	"gopkg.in/yaml.v3"
)

const docsConfigReference = "../docs/reference/configuration.md"

var docsSections = map[reflect.Type]string{
	reflect.TypeFor[SrvConf]():               "",
	reflect.TypeFor[httputil.Conf]():         "## `server_conf`",
	reflect.TypeFor[DAVExportConf]():         "## `dav_export`",
	reflect.TypeFor[AppConf]():               "## `app_conf`",
	reflect.TypeFor[SecurityConf]():          "### `app_conf.security`",
	reflect.TypeFor[WorkerConf]():            "### `app_conf.worker`",
	reflect.TypeFor[shelf.ShelfConfWithID](): "### `shelves[]`",
	reflect.TypeFor[shelf.ShelfConf]():       "### `shelves[]`",
	reflect.TypeFor[shelf.SourceConf]():      "### `shelves[].source`",
	reflect.TypeFor[epub.Strategy]():         "### `app_conf.epub_import_strategy`",
	reflect.TypeFor[logutil.LogConf]():       "## Logger blocks",
	reflect.TypeFor[logutil.LogFileConf]():   "### `log_file`",
}

func TestDocsConfigExampleLoads(t *testing.T) {
	dec := yaml.NewDecoder(bytes.NewReader(configExampleYAML(t)))
	dec.KnownFields(true)

	var conf SrvConf
	if err := dec.Decode(&conf); err != nil {
		t.Fatalf("documented config example does not decode: %v", err)
	}

	if conf.ServerConf == nil {
		t.Error("documented example is missing server_conf")
	}
	if conf.AppConf == nil {
		t.Fatal("documented example is missing app_conf")
	}
	if len(conf.AppConf.Shelves) == 0 {
		t.Error("documented example is missing app_conf.shelves")
	}
	if conf.AppConf.Worker == nil {
		t.Error("documented example is missing app_conf.worker")
	}
	if conf.AppConf.Security == nil {
		t.Fatal("documented example is missing app_conf.security")
	}
	if _, err := NewSecurity(conf.AppConf.Security); err != nil {
		t.Errorf("documented security section is rejected: %v", err)
	}
	if err := ValidateSecurityForListenAddr(conf.AppConf.Security, conf.ServerConf.Addr); err != nil {
		t.Errorf("documented example fails the listen-address security check: %v", err)
	}
}

func TestDocsConfigCoversEveryKey(t *testing.T) {
	sections := docSections(t)

	for _, field := range yamlFields(reflect.TypeFor[SrvConf](), nil) {
		heading, ok := docsSections[field.owner]
		if !ok {
			t.Errorf("configuration struct %s has no section in %s; add one and map it in docsSections",
				field.owner, docsConfigReference)
			continue
		}
		keys, ok := sections[heading]
		if !ok {
			t.Errorf("docsSections points %s at heading %q, which %s does not have",
				field.owner, heading, docsConfigReference)
			continue
		}
		if _, ok := keys[field.key]; !ok {
			t.Errorf("config key %q of %s is not documented under %q in %s",
				field.key, field.owner, headingLabel(heading), docsConfigReference)
		}
	}
}

func TestDocsSectionsAreReachable(t *testing.T) {
	reachable := make(map[reflect.Type]bool)
	for _, field := range yamlFields(reflect.TypeFor[SrvConf](), nil) {
		reachable[field.owner] = true
	}
	for owner := range docsSections {
		if !reachable[owner] {
			t.Errorf("docsSections lists %s, which is no longer reachable from SrvConf", owner)
		}
	}
}

type yamlField struct {
	owner reflect.Type
	key   string
}

func yamlFields(t reflect.Type, seen []reflect.Type) []yamlField {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	if slices.Contains(seen, t) {
		return nil
	}
	seen = append(seen, t)

	var fields []yamlField
	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}

		if name != "" {
			fields = append(fields, yamlField{owner: t, key: name})
		}
		fields = append(fields, yamlFields(field.Type, seen)...)
	}
	return fields
}

var docKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

func docSections(t *testing.T) map[string]map[string]struct{} {
	t.Helper()

	sections := map[string]map[string]struct{}{}
	heading := ""
	keys := map[string]struct{}{}

	for line := range strings.Lines(docPage(t)) {
		line = strings.TrimRight(line, "\n")

		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			sections[heading] = keys
			heading, keys = line, map[string]struct{}{}
			continue
		}
		if key, ok := docTableKey(line); ok {
			keys[key] = struct{}{}
			continue
		}
		if key, _, ok := strings.Cut(strings.TrimLeft(line, " -"), ":"); ok && docKeyPattern.MatchString(key) {
			keys[key] = struct{}{}
		}
	}
	sections[heading] = keys

	return sections
}

func docTableKey(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return "", false
	}
	cell := strings.Trim(strings.TrimPrefix(line, "|"), " ")
	cell, _, _ = strings.Cut(cell, "|")
	cell = strings.Trim(strings.TrimSpace(cell), "`")
	if !docKeyPattern.MatchString(cell) {
		return "", false
	}
	return cell, true
}

func headingLabel(heading string) string {
	if heading == "" {
		return "the page introduction"
	}
	return strings.TrimLeft(heading, "# ")
}

func configExampleYAML(t *testing.T) []byte {
	t.Helper()

	const heading = "\n## A complete example\n"
	_, after, found := strings.Cut(docPage(t), heading)
	if !found {
		t.Fatalf("%s no longer has a %q section", docsConfigReference, strings.TrimSpace(heading))
	}
	_, after, found = strings.Cut(after, "```yaml\n")
	if !found {
		t.Fatalf("%s has no YAML block under the complete example", docsConfigReference)
	}
	block, _, found := strings.Cut(after, "```")
	if !found {
		t.Fatalf("%s has an unterminated YAML block under the complete example", docsConfigReference)
	}
	return []byte(block)
}

func docPage(t *testing.T) string {
	t.Helper()

	page, err := os.ReadFile(docsConfigReference)
	if err != nil {
		t.Fatalf("read %s: %v", docsConfigReference, err)
	}
	return string(page)
}
