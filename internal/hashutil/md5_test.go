package hashutil

import (
	"strings"
	"testing"
)

func TestMD5Hash(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "hello world",
			input:    "Hello, World!",
			expected: "65a8e27d8879283831b664bd8b7f0ad4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := MD5Hash(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("MD5Hash returned an error: %v", err)
			}
			if hash != tc.expected {
				t.Errorf("Expected MD5 hash '%s', got '%s'", tc.expected, hash)
			}
		})
	}
}
