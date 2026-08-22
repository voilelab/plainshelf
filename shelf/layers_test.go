package shelf

import "testing"

func TestNewLayersFromString(t *testing.T) {
	tests := []struct {
		input string
		want  Layers
	}{
		{input: "", want: nil},
		{input: "fiction", want: Layers{"fiction"}},
		{input: " fiction / classics ", want: Layers{"fiction", "classics"}},
	}
	for _, tt := range tests {
		if got := NewLayersFromString(tt.input); !got.Equal(tt.want) {
			t.Errorf("NewLayersFromString(%q) = %#v, want %#v", tt.input, got, tt.want)
		}
	}
}
