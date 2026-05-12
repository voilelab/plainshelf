package util

import (
	"strings"
	"testing"
)

func TestLineCount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty", "", 0, false},
		{"single line", "hello world", 1, false},
		{"multiple lines", "line1\nline2\nline3", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got, err := LineCount(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("LineCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LineCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCharCount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty", "", 0, false},
		{"ASCII", "hello", 5, false},
		{"Unicode", "你好世界", 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got, err := CharCount(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("CharCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CharCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
