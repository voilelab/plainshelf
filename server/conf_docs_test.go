package server

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/httputil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
	"gopkg.in/yaml.v3"
)

// docsConfigReference is the page that documents every configuration key.
// PSW-64 made it the public description of the config file, so it is checked
// here rather than left to drift: a renamed or added key is a change to an
// interface users depend on, and the tests below fail when the page and the
// structs stop agreeing.
const docsConfigReference = "../docs/reference/configuration.md"

// docsSections names the heading whose section documents each configuration
// struct's own keys. The empty string is the page's introduction, above the
// first heading, where the three top-level sections are named.
//
// A key is looked for in its owner's section rather than anywhere on the page,
// so a new key cannot be counted as documented by a same-named key belonging to
// a different section — a `server_conf.type` would not be covered by
// `log_file`'s `type`. A struct reachable from SrvConf and missing from this map
// fails the test, which is what forces a new configuration section to be given
// a section on the page.
var docsSections = map[reflect.Type]string{
	reflect.TypeFor[SrvConf]():               "",
	reflect.TypeFor[httputil.Conf]():         "## `server_conf`",
	reflect.TypeFor[AppConf]():               "## `app_conf`",
	reflect.TypeFor[SecurityConf]():          "### `app_conf.security`",
	reflect.TypeFor[WorkerConf]():            "### `app_conf.worker`",
	reflect.TypeFor[shelf.ShelfConfWithID](): "### `shelves[]`",
	reflect.TypeFor[shelf.ShelfConf]():       "### `shelves[]`",
	reflect.TypeFor[epub.Strategy]():         "### `app_conf.epub_import_strategy`",
	reflect.TypeFor[logutil.LogConf]():       "## Logger blocks",
	reflect.TypeFor[logutil.LogFileConf]():   "### `log_file`",
}

// TestDocsConfigExampleLoads decodes the page's complete example with unknown
// fields rejected, so a key the page spells wrong -- or one the code has since
// dropped -- fails here instead of on a user's first startup.
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

// TestDocsConfigCoversEveryKey walks SrvConf and requires every YAML key to be
// documented in the section that describes the struct it belongs to, so a key
// added to the config structs cannot ship undocumented.
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

// TestDocsSectionsAreReachable keeps docsSections from accumulating entries for
// structs the configuration no longer has.
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

// yamlField is one configuration key and the struct that declares it.
type yamlField struct {
	owner reflect.Type
	key   string
}

// yamlFields collects every field reachable from t that carries a YAML key.
// seen breaks the recursion on a type that appears more than once, which
// LogConf does.
func yamlFields(t reflect.Type, seen []reflect.Type) []yamlField {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	for _, prev := range seen {
		if prev == t {
			return nil
		}
	}
	seen = append(seen, t)

	var fields []yamlField
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		// An empty name is an inline or untagged struct: it contributes no key
		// of its own, only the ones its fields carry.
		if name != "" {
			fields = append(fields, yamlField{owner: t, key: name})
		}
		fields = append(fields, yamlFields(field.Type, seen)...)
	}
	return fields
}

// docKeyPattern is what a configuration key looks like, so that a URL inside a
// YAML list item is not mistaken for one.
var docKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// docSections splits the page at its headings and collects, per section, the
// keys it names: the first column of each table row, and each key line of each
// YAML block. A section ends at the next heading of any level, so a subsection's
// keys are never counted as its parent's.
func docSections(t *testing.T) map[string]map[string]struct{} {
	t.Helper()

	sections := map[string]map[string]struct{}{}
	heading := ""
	keys := map[string]struct{}{}

	for line := range strings.Lines(docPage(t)) {
		line = strings.TrimRight(line, "\n")
		// Only "##" and deeper open a section, so the page title stays part of
		// the introduction and a "#" comment inside a YAML block is not a heading.
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

// docTableKey returns the key a Markdown table row names in its first column.
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

// headingLabel is a heading as a reader would name it, without its Markdown
// level or the introduction's empty string.
func headingLabel(heading string) string {
	if heading == "" {
		return "the page introduction"
	}
	return strings.TrimLeft(heading, "# ")
}

// configExampleYAML returns the YAML block under the page's "A complete
// example" heading.
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
