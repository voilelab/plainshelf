package shelfmobile

// MobileBook is a gomobile-friendly JSON DTO for shelf.Book metadata.
type MobileBook struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Format        string   `json:"format,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Cover         string   `json:"cover,omitempty"`
	Authors       []string `json:"authors,omitempty"`
	Language      string   `json:"language,omitempty"`
	Comments      string   `json:"comments,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	PublishedAt   string   `json:"published_at,omitempty"`
	CurrentSource string   `json:"current_source,omitempty"`
	Layers        []string `json:"layers,omitempty"`
}

// CreateBookRequest is the JSON payload accepted by CreateBookJSON.
type CreateBookRequest struct {
	Title  string   `json:"title"`
	Layers []string `json:"layers,omitempty"`
}

// UpdateBookRequest is the JSON patch payload accepted by UpdateBookJSON.
type UpdateBookRequest struct {
	Title       *string   `json:"title,omitempty"`
	Authors     *[]string `json:"authors,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Language    *string   `json:"language,omitempty"`
	Comments    *string   `json:"comments,omitempty"`
	PublishedAt *string   `json:"published_at,omitempty"`
}

// MoveBookRequest is the JSON payload accepted by MoveBookJSON.
type MoveBookRequest struct {
	Layers []string `json:"layers"`
}

// MobileSource is a gomobile-friendly JSON DTO for shelf.Source metadata.
type MobileSource struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at,omitempty"`
	Comment   string `json:"comment,omitempty"`
	MD5Hash   string `json:"md5_hash,omitempty"`
	LineCount int    `json:"line_count,omitempty"`
	CharCount int    `json:"char_count,omitempty"`
}
