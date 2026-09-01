package shelf

import (
	"errors"
	"sync"

	"github.com/voilelab/plainshelf/internal/util"
)

type ShelfData struct {
	ID   string
	Name string

	// conf is the configuration this shelf was opened with. UpdateShelf needs
	// it to tell an in-place change from one that can only be applied by
	// opening the shelf again, and to reopen the previous one when that fails.
	conf ShelfConf

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

// UpdateShelf applies a new configuration to a shelf that is already open.
//
// The name, the scan interval and the per-book check interval are changed on
// the live shelf. read_only cannot be: it is read while the shelf is being
// opened - it picks the lock
// mode, decides whether lib_root may be created, whether app/ is written and
// whether the book cache is exported - so changing it closes the shelf and
// opens it again from the new configuration.
//
// lib_root is not a setting this can change; a shelf that should live somewhere
// else is removed and added again under the new path.
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

	if conf.ReadOnly != s.conf.ReadOnly {
		return sm.reopenShelfLocked(s, conf)
	}

	if err := s.SetScanInterval(conf.ScanInterval); err != nil {
		return util.Errorf("%w", err)
	}

	// After SetScanInterval, so an empty book_check_interval ("same as scan
	// interval") follows the interval this same update just applied.
	if err := s.SetBookCheckInterval(conf.BookCheckInterval); err != nil {
		return util.Errorf("%w", err)
	}

	s.Name = conf.Name
	s.conf = conf.ShelfConf
	return nil
}

// reopenShelfLocked replaces an open shelf with one opened from conf.
//
// The old shelf is closed first: both configurations name the same lib_root,
// and a writable shelf opened next to the one it is replacing would contend
// with it for the lock file, the scan cache and the exported book cache.
//
// That leaves a window with nothing open, so a configuration that does not open
// - read_only pointed at a lib_root that no longer exists - falls back to the
// previous one instead of leaving the shelf unregistered. Only when that also
// fails to open is the shelf dropped, and the error says so.
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
