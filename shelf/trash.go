package shelf

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

const trashMetaFile = "trash.json"
const maxBookPathCollisionAttempts = 10000

var ErrTrashedBookNotFound = util.NewError("trashed book not found")

type TrashedBook struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Authors       []string      `json:"authors"`
	OriginalPath  string        `json:"original_path,omitempty"`
	OriginalLayer Layers        `json:"original_layer,omitempty"`
	DeletedAt     util.JSONTime `json:"deleted_at,omitzero"`
}

type trashMeta struct {
	DeletedAt     util.JSONTime `json:"deleted_at,omitzero"`
	OriginalPath  string        `json:"original_path,omitempty"`
	OriginalLayer Layers        `json:"original_layer,omitempty"`
	DeleteReason  string        `json:"delete_reason,omitempty"`
}

func (s *Shelf) MoveBookToTrash(bookID string) error {
	s.lock()
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	activePath := book.FolderPath()
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)

	if _, err := s.dbRoot.Stat(trashPath); err == nil {
		return util.Errorf("book %q already exists in trash", bookID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.Rename(activePath, trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	meta := trashMeta{
		DeletedAt:     util.JSONTime(time.Now()),
		OriginalPath:  activePath,
		OriginalLayer: append(Layers(nil), book.Layers()...),
		DeleteReason:  "user",
	}
	if err := s.writeTrashMeta(trashPath, &meta); err != nil {
		_ = s.dbRoot.Rename(trashPath, activePath)
		return util.Errorf("%w", err)
	}

	s.deleteBookCacheEntry(bookID)
	return nil
}

func (s *Shelf) ListTrashedBooks() ([]*TrashedBook, error) {
	s.rlock()
	defer s.unlock()

	entries, err := s.dbRoot.ReadDir(trashBooksFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}

	items := make([]*TrashedBook, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
			continue
		}

		bookPath := path.Join(trashBooksFolder, entry.Name())
		book, err := openBook(s.dbRoot, s.Logger, bookPath)
		if err != nil {
			s.Warn("failed to open trashed book, skipping", "path", bookPath, "error", err)
			continue
		}

		meta, err := s.readTrashMeta(bookPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			s.Warn("failed to read trash metadata, skipping", "path", bookPath, "error", err)
			continue
		}

		item := &TrashedBook{
			ID:      book.ID(),
			Title:   book.Title(),
			Authors: append([]string(nil), book.GetMeta().Authors...),
		}
		if meta != nil {
			item.DeletedAt = meta.DeletedAt
			item.OriginalPath = meta.OriginalPath
			item.OriginalLayer = append(Layers(nil), meta.OriginalLayer...)
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].DeletedAt != items[j].DeletedAt {
			return time.Time(items[i].DeletedAt).After(time.Time(items[j].DeletedAt))
		}
		return items[i].ID < items[j].ID
	})

	return items, nil
}

func (s *Shelf) RestoreTrashedBook(bookID string) error {
	s.lock()
	defer s.unlock()

	trashPath, book, meta, err := s.findTrashedBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	targetLayers := Layers(nil)
	targetFolder := path.Base(trashPath)
	if meta != nil {
		targetLayers = append(Layers(nil), meta.OriginalLayer...)
		if base := path.Base(meta.OriginalPath); strings.HasSuffix(base, bookExtension) {
			targetFolder = base
		}
	}

	if err := validateLayers(targetLayers); err != nil {
		targetLayers = nil
	}

	targetLayerPath := path.Join(booksFolder, path.Join(targetLayers...))
	if err := s.dbRoot.MkdirAll(targetLayerPath); err != nil {
		return util.Errorf("%w", err)
	}

	targetPath, err := s.resolveBookPathCollision(targetLayerPath, targetFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.Rename(trashPath, targetPath); err != nil {
		return util.Errorf("%w", err)
	}
	_ = s.dbRoot.Remove(path.Join(targetPath, trashMetaFile))

	restoredBook, err := openBook(s.dbRoot, s.Logger, targetPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	restoredBook.setLayers(targetLayers)
	s.updateBookCacheEntry(restoredBook.Layers(), targetPath, restoredBook)

	if restoredBook.ID() != book.ID() {
		return util.Errorf("restored book id mismatch")
	}

	return nil
}

func (s *Shelf) DeleteTrashedBook(bookID string) error {
	s.lock()
	defer s.unlock()

	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)
	if _, err := s.dbRoot.Stat(trashPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("%w", ErrTrashedBookNotFound)
		}
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.RemoveAll(trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) findTrashedBook(bookID string) (string, *Book, *trashMeta, error) {
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)
	book, err := openBook(s.dbRoot, s.Logger, trashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil, util.Errorf("%w", ErrTrashedBookNotFound)
		}
		return "", nil, nil, util.Errorf("%w", err)
	}

	meta, err := s.readTrashMeta(trashPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, nil, util.Errorf("%w", err)
	}

	return trashPath, book, meta, nil
}

func (s *Shelf) writeTrashMeta(bookPath string, meta *trashMeta) error {
	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := s.dbRoot.WriteFile(path.Join(bookPath, trashMetaFile), payload); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (s *Shelf) readTrashMeta(bookPath string) (*trashMeta, error) {
	fp, err := s.dbRoot.Open(path.Join(bookPath, trashMetaFile))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer fp.Close()

	var meta trashMeta
	if err := json.NewDecoder(fp).Decode(&meta); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &meta, nil
}

func (s *Shelf) resolveBookPathCollision(layerPath, folderName string) (string, error) {
	baseName := strings.TrimSuffix(folderName, bookExtension)
	if baseName == "" {
		baseName = folderName
	}

	// maxBookPathCollisionAttempts is a practical upper bound for collision resolution in a single layer.
	// If the bound is reached, return an error instead of looping indefinitely.
	for i := 0; i < maxBookPathCollisionAttempts; i++ {
		candidateFolder := folderName
		if i > 0 {
			candidateFolder = baseName + "-" + strconv.Itoa(i) + bookExtension
		}
		candidatePath := path.Join(layerPath, candidateFolder)
		_, err := s.dbRoot.Stat(candidatePath)
		if errors.Is(err, os.ErrNotExist) {
			return candidatePath, nil
		}
		if err != nil {
			return "", util.Errorf("%w", err)
		}
	}

	return "", util.Errorf("failed to resolve unique book path for %q", folderName)
}
