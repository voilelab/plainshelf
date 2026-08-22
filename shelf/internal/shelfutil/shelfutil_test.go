package shelfutil

import "testing"

func TestValidateBCP47(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"en", true},
		{"zh-Hant", true},
		{"fr-CA", true},
		{"es-419", true},
		{"invalid-lang", false},
		{"123", false},
	}

	for _, test := range tests {
		result := ValidateBCP47(test.lang)
		if result != test.expected {
			t.Errorf("ValidateBCP47(%q) = %v; expected %v", test.lang, result, test.expected)
		}
	}
}
