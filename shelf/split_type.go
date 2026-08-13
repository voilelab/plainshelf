package shelf

type SplitType string

const (
	SplitTypeNone      SplitType = ""
	SplitTypeLineCount SplitType = "line_count"
	SplitTypeRegex     SplitType = "regex"
	SplitTypeBoundary  SplitType = "boundary"
)

// SplitConfig is the persisted chapter model for legacy sources only.
// Schema-versioned TXT/Markdown sources ignore it; Markdown chapters are H2s.
// Deprecated: keep for legacy shelves and older clients until their upgrade
// window closes.
type SplitConfig struct {
	// the method to split the novel into parts. Default is "none", which means no splitting (the novel is one part).
	Type SplitType `json:"type"`

	// Split the novel into parts with fixed line count. Required if Type is "line_count".
	LineCount int `json:"line_count,omitempty"`

	// Split the novel into parts based on regex matches. Required if Type is "regex".
	Regex string `json:"regex,omitempty"`

	// Split the novel into parts based on line numbers. Required if Type is "boundary".
	Boundaries []int `json:"boundaries,omitempty"`
}
