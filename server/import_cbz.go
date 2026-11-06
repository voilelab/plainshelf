package server

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/cbz"
	"github.com/voilelab/plainshelf/shelf"
)

type cbzInputError struct{ cause error }

func (e *cbzInputError) Error() string { return e.cause.Error() }
func (e *cbzInputError) Unwrap() error { return e.cause }

func (h *importHandlers) importCBZ(data *shelf.ShelfData, r io.ReaderAt, size int64, filename, title string, folders shelf.FolderPath) (*shelf.Book, error) {
	parsed, err := cbz.Open(r, size)
	if err != nil {
		return nil, &cbzInputError{err}
	}
	dir, err := os.MkdirTemp("", "plainshelf-cbz-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	names := make([]string, 0, len(parsed.Pages))
	for _, page := range parsed.Pages {
		image, err := page.Read()
		if err != nil {
			return nil, &cbzInputError{err}
		}
		if err := os.WriteFile(filepath.Join(dir, page.Name), image, 0600); err != nil {
			return nil, err
		}
		names = append(names, page.Name)
	}
	title = strings.TrimSpace(title)
	if title == "" || strings.EqualFold(title, filename) || strings.EqualFold(title, deriveTitleFromFilename(filename)) {
		title = parsed.Title
		if title == "" {
			title = deriveTitleFromFilename(filename)
		}
	}
	return data.NewBookWith(folders, title, func(book *shelf.Book) error {
		source, err := book.NewSourceWithOptions(strings.NewReader(strings.Join(names, "\n")+"\n"), shelf.NewSourceOptions{Format: shelf.BookFormatImages, PageDirection: parsed.Direction})
		if err != nil {
			return err
		}
		for i, name := range names {
			image, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			if err := source.WriteAsset(name, image); err != nil {
				return err
			}
			if i == 0 {
				if err := book.SetCover(image, filepath.Ext(name)); err != nil {
					return err
				}
			}
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return err
		}
		meta := book.GetMeta()
		meta.Authors = parsed.Authors
		return book.SetMeta(meta)
	})
}
