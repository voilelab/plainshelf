package main

import (
	"encoding/xml"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

const bookpkgUTI = "com.voilelab.plainshelf.bookpkg"

func decodePlistValue(t *testing.T, dec *xml.Decoder, start xml.StartElement) any {
	t.Helper()
	switch start.Name.Local {
	case "true", "false":
		if err := dec.Skip(); err != nil {
			t.Fatal(err)
		}
		return start.Name.Local == "true"
	case "string":
		var s string
		if err := dec.DecodeElement(&s, &start); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(s)
	case "array", "dict":
		var items []any
		dict := map[string]any{}
		key := ""
		for {
			tok, err := dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			switch el := tok.(type) {
			case xml.EndElement:
				if start.Name.Local == "array" {
					return items
				}
				return dict
			case xml.StartElement:
				if el.Name.Local == "key" {
					if err := dec.DecodeElement(&key, &el); err != nil {
						t.Fatal(err)
					}
					continue
				}
				value := decodePlistValue(t, dec, el)
				if start.Name.Local == "array" {
					items = append(items, value)
				} else {
					dict[key] = value
				}
			}
		}
	}
	t.Fatalf("unsupported plist element %q", start.Name.Local)
	return nil
}

func readInfoPlist(t *testing.T) map[string]any {
	t.Helper()
	f, err := os.Open("build/darwin/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			t.Fatal("Info.plist has no top-level dict")
		}
		if err != nil {
			t.Fatal(err)
		}
		if el, ok := tok.(xml.StartElement); ok && el.Name.Local == "dict" {
			return decodePlistValue(t, dec, el).(map[string]any)
		}
	}
}

func firstDict(t *testing.T, plist map[string]any, key string) map[string]any {
	t.Helper()
	items, _ := plist[key].([]any)
	if len(items) == 0 {
		t.Fatalf("Info.plist %s is missing or empty", key)
	}
	return items[0].(map[string]any)
}

func TestInfoPlistDeclaresBookpkgPackage(t *testing.T) {
	plist := readInfoPlist(t)

	uti := firstDict(t, plist, "UTExportedTypeDeclarations")
	if uti["UTTypeIdentifier"] != bookpkgUTI {
		t.Fatalf("exported UTI = %v, want %s", uti["UTTypeIdentifier"], bookpkgUTI)
	}
	conforms, _ := uti["UTTypeConformsTo"].([]any)
	if !slices.Contains(conforms, any("com.apple.package")) {
		t.Fatalf("UTI conforms to %v; without com.apple.package Finder shows a folder", conforms)
	}
	tags, _ := uti["UTTypeTagSpecification"].(map[string]any)
	if exts, _ := tags["public.filename-extension"].([]any); !slices.Contains(exts, any("bookpkg")) {
		t.Fatalf("UTI extensions = %v, want bookpkg", exts)
	}

	doc := firstDict(t, plist, "CFBundleDocumentTypes")
	types, _ := doc["LSItemContentTypes"].([]any)
	if !slices.Contains(types, any(bookpkgUTI)) || doc["LSTypeIsPackage"] != true || doc["CFBundleTypeRole"] != "Viewer" {
		t.Fatalf("document type = %v, want a package viewer for %s", doc, bookpkgUTI)
	}
	if plist["CFBundleIdentifier"] != singleInstanceID {
		t.Fatalf("CFBundleIdentifier = %v, want the single-instance id %s", plist["CFBundleIdentifier"], singleInstanceID)
	}
}
