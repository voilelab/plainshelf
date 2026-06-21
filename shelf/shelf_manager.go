package shelf

import (
	"errors"
	"sync"

	"github.com/voilelab/plainshelf/internal/util"
)

type ShelfData struct {
	ID   string
	Name string
	*Shelf
}

type ShelfConfWithID struct {
	// ID is a unique identifier for the shelf. It should be uri-safe as it may be used in URLs.
	// It is used in the API to specify which shelf to use.
	ID string `yaml:"id"`

	// Name is a human-readable name for the shelf.
	// If not provided, it will default to the same value as ID.
	Name string `yaml:"name"`

	ShelfConf `yaml:",inline"`
}

type ShelfManager struct {
	shelves map[string]*ShelfData
	lock    sync.RWMutex
}

func NewShelfManager() *ShelfManager {
	return &ShelfManager{
		shelves: make(map[string]*ShelfData),
	}
}

func (sm *ShelfManager) AddShelf(conf ShelfConfWithID) error {
	if conf.ID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	if _, exists := sm.shelves[conf.ID]; exists {
		return util.Errorf("duplicate shelf ID: %q", conf.ID)
	}

	s, err := NewShelf(&conf.ShelfConf)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if conf.Name == "" {
		conf.Name = conf.ID
	}

	sm.shelves[conf.ID] = &ShelfData{
		Shelf: s,
		Name:  conf.Name,
		ID:    conf.ID,
	}

	return nil
}

func (sm *ShelfManager) Close() error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	errs := make([]error, 0)

	for _, s := range sm.shelves {
		if e := s.Close(); e != nil {
			errs = append(errs, e)
		}
	}

	err := errors.Join(errs...)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (sm *ShelfManager) GetShelf(id string) (*ShelfData, bool) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	s, exists := sm.shelves[id]
	return s, exists
}

func (sm *ShelfManager) GetAllShelves() []ShelfData {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	shelves := make([]ShelfData, 0, len(sm.shelves))
	for _, s := range sm.shelves {
		shelves = append(shelves, *s)
	}
	return shelves
}

func (sm *ShelfManager) UpdateShelf(id, name, scanInterval string) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	s, exists := sm.shelves[id]
	if !exists {
		return util.Errorf("shelf with ID %q does not exist", id)
	}

	if err := s.SetScanInterval(scanInterval); err != nil {
		return util.Errorf("%w", err)
	}

	s.Name = name
	return nil
}

func (sm *ShelfManager) RemoveShelf(id string) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	s, exists := sm.shelves[id]
	if !exists {
		return util.Errorf("shelf with ID %q does not exist", id)
	}

	if err := s.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	delete(sm.shelves, id)
	return nil
}
