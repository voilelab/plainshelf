package guiutil

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"golang.org/x/net/webdav"
)

func TestParseLibConfWithURI(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	validURI := storage.NewFileURI(t.TempDir()).String()

	type testCase struct {
		name        string
		confBs      []byte
		expectError bool
	}

	tests := []testCase{
		{
			name: "valid URI config",
			confBs: fmt.Appendf(nil, `{
				"type": "uri",
				"conf": {
					"uri": %q
				}
			}`, validURI),
			expectError: false,
		},
		{
			name: "invalid URI config (wrong type)",
			confBs: []byte(`{
				"type": "uri",
				"conf": "not a struct"
			}`),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var conf LibConf
			err := json.Unmarshal(tc.confBs, &conf)
			if err != nil {
				t.Fatalf("failed to unmarshal test config: %v", err)
			}

			_, err = NewLib(&conf)
			if tc.expectError && err == nil {
				t.Errorf("expected error but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseLibConfWithWebDAV(t *testing.T) {
	server := httptest.NewServer(&webdav.Handler{
		FileSystem: webdav.NewMemFS(),
		LockSystem: webdav.NewMemLS(),
	})
	defer server.Close()

	type testCase struct {
		name        string
		confBs      []byte
		expectError bool
	}

	tests := []testCase{
		{
			name: "valid WebDAV config",
			confBs: fmt.Appendf(nil, `{
				"type": "webdav",
				"conf": {
					"host": %q,
					"user": "user",
					"password": "pass"
				}
			}`, server.URL),
			expectError: false,
		},
		{
			name: "invalid WebDAV config (wrong type)",
			confBs: []byte(`{
				"type": "webdav",
				"conf": "not a struct"
			}`),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var conf LibConf
			err := json.Unmarshal(tc.confBs, &conf)
			if err != nil {
				t.Fatalf("failed to unmarshal test config: %v", err)
			}

			_, err = NewLib(&conf)
			if tc.expectError && err == nil {
				t.Errorf("expected error but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
