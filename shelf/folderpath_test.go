package shelf

import "testing"

func TestNewFolderPathFromString(t *testing.T) {
	tests := []struct {
		input string
		want  FolderPath
	}{
		{input: "", want: nil},
		{input: "fiction", want: FolderPath{"fiction"}},
		{input: " fiction / classics ", want: FolderPath{"fiction", "classics"}},
	}
	for _, tt := range tests {
		if got := NewFolderPathFromString(tt.input); !got.Equal(tt.want) {
			t.Errorf("NewFolderPathFromString(%q) = %#v, want %#v", tt.input, got, tt.want)
		}
	}
}
