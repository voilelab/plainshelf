package shelf

import (
	"errors"
	"sync"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type ShelfData struct {
	ID   string
	Name string

	conf ShelfConf

	*Shelf
}

type ShelfConfWithID struct {
	ID string `yaml:"id"`

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
		conf:  conf.ShelfConf,
	}

	return nil
}

func (sm *ShelfManager) AddVirtualShelf(conf ShelfConfWithID, root fsutil.ReadFS, offlineDir string) error {
	if conf.ID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	if _, exists := sm.shelves[conf.ID]; exists {
		return util.Errorf("duplicate shelf ID: %q", conf.ID)
	}

	s, err := NewShelfFromFS(&conf.ShelfConf, root, offlineDir)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if conf.Name == "" {
		conf.Name = conf.ID
	}

	conf.ReadOnly = true
	sm.shelves[conf.ID] = &ShelfData{
		Shelf: s,
		Name:  conf.Name,
		ID:    conf.ID,
		conf:  conf.ShelfConf,
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

func (sm *ShelfManager) UpdateShelf(conf ShelfConfWithID) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	s, exists := sm.shelves[conf.ID]
	if !exists {
		return util.Errorf("shelf with ID %q does not exist", conf.ID)
	}

	if conf.LibRoot != s.conf.LibRoot {
		return util.Errorf("shelf %q lib_root cannot be changed from %q to %q", conf.ID, s.conf.LibRoot, conf.LibRoot)
	}
	if (conf.Source == nil) != (s.conf.Source == nil) || (conf.Source != nil && *conf.Source != *s.conf.Source) {
		return util.Errorf("shelf %q source cannot be changed", conf.ID)
	}

	if conf.ReadOnly != s.conf.ReadOnly {
		if s.IsVirtual() {
			return util.Errorf("shelf %q is virtual and always read-only", conf.ID)
		}
		return sm.reopenShelfLocked(s, conf)
	}

	if err := s.SetIntervals(conf.ScanInterval, conf.BookCheckInterval); err != nil {
		return util.Errorf("%w", err)
	}

	s.Name = conf.Name
	s.conf = conf.ShelfConf
	return nil
}

func (sm *ShelfManager) reopenShelfLocked(s *ShelfData, conf ShelfConfWithID) error {
	previous := s.conf

	if err := s.Close(); err != nil {
		return util.Errorf("closing shelf %q: %w", conf.ID, err)
	}

	next, err := NewShelf(&conf.ShelfConf)
	if err != nil {
		restored, restoreErr := NewShelf(&previous)
		if restoreErr != nil {
			delete(sm.shelves, conf.ID)
			return util.Errorf("reopening shelf %q: %w; the previous configuration no longer opens either (%v), so the shelf is now closed", conf.ID, err, restoreErr)
		}
		s.Shelf = restored
		return util.Errorf("reopening shelf %q: %w", conf.ID, err)
	}

	s.Shelf = next
	s.Name = conf.Name
	s.conf = conf.ShelfConf
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
