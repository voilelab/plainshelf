package shelfmobile

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// Shelf is a gomobile-friendly facade over the Go-native shelf core.
type Shelf struct {
	inner *shelf.Shelf
}

// Open opens or creates a shelf library at libRoot using quiet mobile logging.
func Open(libRoot string) (*Shelf, error) {
	inner, err := shelf.NewShelf(&shelf.ShelfConf{
		LibRoot: libRoot,
		Logger: logutil.LogConf{
			LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
		},
	})
	if err != nil {
		return nil, err
	}
	return &Shelf{inner: inner}, nil
}

func (s *Shelf) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *Shelf) ListBooksJSON() (string, error) {
	books, err := s.inner.ListBooks()
	if err != nil {
		return "", err
	}
	out := make([]MobileBook, 0, len(books))
	for _, book := range books {
		out = append(out, mobileBook(book))
	}
	return marshalString(out)
}

func (s *Shelf) GetBookJSON(bookID string) (string, error) {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return "", err
	}
	return marshalString(mobileBook(book))
}

func (s *Shelf) CreateBookJSON(reqJSON string) (string, error) {
	var req CreateBookRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return "", err
	}
	book, err := s.inner.NewBook(shelf.Layers(req.Layers), req.Title)
	if err != nil {
		return "", err
	}
	return marshalString(mobileBook(book))
}

func (s *Shelf) UpdateBookJSON(bookID string, patchJSON string) (string, error) {
	var req UpdateBookRequest
	if err := json.Unmarshal([]byte(patchJSON), &req); err != nil {
		return "", err
	}
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return "", err
	}
	meta := book.GetMeta()
	if req.Title != nil {
		meta.Title = *req.Title
	}
	if req.Authors != nil {
		meta.Authors = append([]string(nil), (*req.Authors)...)
	}
	if req.Tags != nil {
		meta.Tags = append([]string(nil), (*req.Tags)...)
	}
	if req.Language != nil {
		meta.Language = *req.Language
	}
	if req.Comments != nil {
		meta.Comments = *req.Comments
	}
	if req.PublishedAt != nil {
		publishedAt, err := time.Parse(time.RFC3339, *req.PublishedAt)
		if err != nil {
			return "", err
		}
		meta.PublishedAt = util.JSONTime(publishedAt)
	}
	if err := book.SetMeta(meta); err != nil {
		return "", err
	}
	return marshalString(mobileBook(book))
}

func (s *Shelf) MoveBookJSON(bookID string, reqJSON string) (string, error) {
	var req MoveBookRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return "", err
	}
	book, err := s.inner.MoveBook(bookID, shelf.Layers(req.Layers))
	if err != nil {
		return "", err
	}
	return marshalString(mobileBook(book))
}

func (s *Shelf) MoveBookToTrash(bookID string) error {
	return s.inner.MoveBookToTrash(bookID)
}

func (s *Shelf) ListSourcesJSON(bookID string) (string, error) {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return "", err
	}
	sources, err := book.ListSource()
	if err != nil {
		return "", err
	}
	out := make([]MobileSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, mobileSource(source))
	}
	return marshalString(out)
}

func (s *Shelf) CreateSourceJSON(bookID string) (string, error) {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return "", err
	}
	source, err := book.NewSource(nil)
	if err != nil {
		return "", err
	}
	if err := book.SetCurrentSource(source.ID()); err != nil {
		return "", err
	}
	return marshalString(mobileSource(source))
}

func (s *Shelf) SetCurrentSource(bookID string, sourceID string) error {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return err
	}
	return book.SetCurrentSource(sourceID)
}

func (s *Shelf) GetSourceContent(bookID string, sourceID string) (string, error) {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return "", err
	}
	source, err := book.GetSource(sourceID)
	if err != nil {
		return "", err
	}
	fp, err := source.Open()
	if err != nil {
		return "", err
	}
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (s *Shelf) UpdateSourceContent(bookID string, sourceID string, content string) error {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return err
	}
	source, err := book.GetSource(sourceID)
	if err != nil {
		return err
	}
	return source.UpdateContent(strings.NewReader(content))
}

func (s *Shelf) GetCover(bookID string) ([]byte, error) {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return nil, err
	}
	image, _, err := book.OpenCover()
	return image, err
}

func (s *Shelf) SetCover(bookID string, image []byte, ext string) error {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return err
	}
	return book.SetCover(image, ext)
}

func (s *Shelf) DeleteCover(bookID string) error {
	book, err := s.inner.GetBook(bookID)
	if err != nil {
		return err
	}
	return book.DeleteCover()
}

func mobileBook(book *shelf.Book) MobileBook {
	meta := book.GetMeta()
	return MobileBook{
		ID:            meta.ID,
		Title:         meta.Title,
		Format:        meta.Format,
		Tags:          append([]string(nil), meta.Tags...),
		Cover:         meta.Cover,
		Authors:       append([]string(nil), meta.Authors...),
		Language:      meta.Language,
		Comments:      meta.Comments,
		CreatedAt:     jsonTimeString(meta.CreatedAt),
		UpdatedAt:     jsonTimeString(meta.UpdatedAt),
		PublishedAt:   jsonTimeString(meta.PublishedAt),
		CurrentSource: meta.CurrentSource,
		Layers:        append([]string(nil), book.Layers()...),
	}
}

func mobileSource(source *shelf.Source) MobileSource {
	meta := source.GetMeta()
	return MobileSource{
		ID:        meta.ID,
		CreatedAt: jsonTimeString(meta.CreatedAt),
		Comment:   meta.Comment,
		MD5Hash:   meta.MD5Hash,
		LineCount: meta.LineCount,
		CharCount: meta.CharCount,
	}
}

func jsonTimeString(t util.JSONTime) string {
	v := time.Time(t)
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339)
}

func marshalString(v any) (string, error) {
	bs, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
