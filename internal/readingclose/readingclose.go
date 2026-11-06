package readingclose

import (
	"log"
	"maps"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/readingprogress"
)

const DefaultTimeout = 1500 * time.Millisecond

type Stager struct {
	store   *readingprogress.Store
	timeout time.Duration

	mu     sync.Mutex
	staged map[string]map[string]readingprogress.Entry
}

func NewStager(store *readingprogress.Store, timeout time.Duration) *Stager {
	return &Stager{
		store:   store,
		timeout: timeout,
		staged:  map[string]map[string]readingprogress.Entry{},
	}
}

func (s *Stager) Stage(shelfID, bookID string, offset, at int64) {
	if s == nil || shelfID == "" || bookID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	books := s.staged[shelfID]
	if books == nil {
		books = map[string]readingprogress.Entry{}
		s.staged[shelfID] = books
	}
	books[bookID] = readingprogress.Entry{Offset: offset, At: at}
}

func (s *Stager) PersistOnClose() {
	if s == nil {
		return
	}
	s.persistWithin(s.snapshot())
}

func (s *Stager) PersistBookOnClose(shelfID, bookID string) {
	if s == nil {
		return
	}
	s.persistWithin(s.bookSnapshot(shelfID, bookID))
}

func (s *Stager) persistWithin(staged readingprogress.Document) {
	done := make(chan struct{})
	go func() {
		s.persist(staged)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.timeout):
	}
}

func (s *Stager) persist(staged readingprogress.Document) {
	if s.store == nil {
		return
	}
	if len(staged.Shelves) == 0 {
		return
	}
	if _, err := s.store.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeNewest(disk, staged)
	}); err != nil {
		log.Println("failed to persist reading progress on close:", err)
	}
}

func (s *Stager) snapshot() readingprogress.Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := readingprogress.New()
	for shelfID, books := range s.staged {
		if len(books) == 0 {
			continue
		}
		doc.Shelves[shelfID] = maps.Clone(books)
	}
	return doc
}

func (s *Stager) bookSnapshot(shelfID, bookID string) readingprogress.Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := readingprogress.New()
	if entry, ok := s.staged[shelfID][bookID]; ok {
		doc.Shelves[shelfID] = map[string]readingprogress.Entry{bookID: entry}
	}
	return doc
}
