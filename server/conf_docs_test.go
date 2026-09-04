package server

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// docsConfigReference is the page that documents every configuration key.
// PSW-64 made it the public description of the config file, so it is checked
// here rather than left to drift: a renamed or added key is a change to an
// interface users depend on, and the two tests below fail when the page and the
// structs stop agreeing.
const docsConfigReference = "../docs/reference/configuration.md"

// TestDocsConfigExampleLoads decodes the page's complete example with unknown
// fields rejected, so a key the page spells wrong -- or one the code has since
// dropped -- fails here instead of on a user's first startup.
func TestDocsConfigExampleLoads(t *testing.T) {
	example := configExampleYAML(t)

	dec := yaml.NewDecoder(bytes.NewReader(example))
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
// named on the reference page, so a key added to the config structs cannot ship
// undocumented.
func TestDocsConfigCoversEveryKey(t *testing.T) {
	page, err := os.ReadFile(docsConfigReference)
	if err != nil {
		t.Fatalf("read %s: %v", docsConfigReference, err)
	}
	doc := string(page)

	for _, key := range yamlKeys(reflect.TypeFor[SrvConf](), nil) {
		if !documentsKey(doc, key) {
			t.Errorf("config key %q is not documented in %s", key, docsConfigReference)
		}
	}
}

// documentsKey reports whether the page names key, either as an inline code
// span in a table or as a key line in the example. A bare substring match is
// not enough: short keys such as "id" and "type" occur inside ordinary words.
func documentsKey(doc, key string) bool {
	if strings.Contains(doc, "`"+key+"`") {
		return true
	}
	for line := range strings.Lines(doc) {
		if strings.HasPrefix(strings.TrimLeft(line, " -"), key+":") {
			return true
		}
	}
	return false
}

// yamlKeys collects the YAML key of every field reachable from t. seen breaks
// the recursion on a type that appears more than once, which LogConf does.
func yamlKeys(t reflect.Type, seen []reflect.Type) []string {
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

	var keys []string
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		// An empty name is an inline or untagged struct: it contributes no key
		// of its own, only the ones its fields carry.
		if name == "-" {
			continue
		}
		if name != "" {
			keys = append(keys, name)
		}
		keys = append(keys, yamlKeys(field.Type, seen)...)
	}
	return keys
}

// configExampleYAML returns the YAML block under the page's "A complete
// example" heading.
func configExampleYAML(t *testing.T) []byte {
	t.Helper()

	page, err := os.ReadFile(docsConfigReference)
	if err != nil {
		t.Fatalf("read %s: %v", docsConfigReference, err)
	}

	const heading = "\n## A complete example\n"
	_, after, found := strings.Cut(string(page), heading)
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
