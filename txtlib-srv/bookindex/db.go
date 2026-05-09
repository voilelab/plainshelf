package bookindex

import (
	"encoding/json"
	"log"
	"maps"
	"sync"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

type MetaMap map[string]string

type Item struct {
	id   string
	meta MetaMap
}

type DB struct {
	items map[string]Item

	// metaKey -> metaVal -> IDs
	revMap map[string]map[string]*util.Set[string]
	bdb    *badger.DB

	lock sync.RWMutex
}

const badgerStateKey = "bookindex:state"

type persistedState struct {
	Items map[string]MetaMap `json:"items"`
}

func New(dbPath string) (*DB, error) {
	opts := badger.DefaultOptions(dbPath).WithLogger(nil)
	bdb, err := badger.Open(opts)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	db := &DB{
		items:  make(map[string]Item),
		revMap: make(map[string]map[string]*util.Set[string]),
		bdb:    bdb,
	}

	if err := db.load(); err != nil {
		bdb.Close()
		return nil, util.Errorf("%w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if err := db.saveLocked(); err != nil {
		return util.Errorf("%w", err)
	}

	err := db.bdb.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// Add adds a new item to the database. It returns false if an item with the same ID already exists.
func (db *DB) Add(id string, meta MetaMap) bool {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.add(id, meta)
}

// Has checks if an item with the given ID exists in the database.
func (db *DB) Has(id string) bool {
	db.lock.RLock()
	defer db.lock.RUnlock()
	_, exists := db.items[id]
	return exists
}

// Remove removes an item from the database. It returns false if the item does not exist.
func (db *DB) Remove(id string) bool {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.remove(id)
}

// Update updates an existing item in the database.
func (db *DB) Update(id string, meta MetaMap) {
	db.lock.Lock()
	defer db.lock.Unlock()
	db.update(id, meta)
}

// add adds a new item to the database. It returns false if an item with the same ID already exists.
func (db *DB) add(id string, meta MetaMap) bool {
	if !db.addNoPersist(id, meta) {
		return false
	}

	if err := db.saveLocked(); err != nil {
		log.Printf("bookindex: failed to persist add for %s: %v", id, err)
	}
	return true
}

func (db *DB) addNoPersist(id string, meta MetaMap) bool {
	if _, exists := db.items[id]; exists {
		return false
	}

	metaCopy := copyMeta(meta)
	item := Item{id: id, meta: metaCopy}
	db.items[id] = item

	for k, v := range metaCopy {
		if _, exists := db.revMap[k]; !exists {
			db.revMap[k] = make(map[string]*util.Set[string])
		}
		if _, exists := db.revMap[k][v]; !exists {
			db.revMap[k][v] = util.NewSet[string]()
		}
		db.revMap[k][v].Add(id)
	}
	return true
}

// remove removes an item from the database. It returns false if the item does not exist.
func (db *DB) remove(id string) bool {
	if !db.removeNoPersist(id) {
		return false
	}

	if err := db.saveLocked(); err != nil {
		log.Printf("bookindex: failed to persist remove for %s: %v", id, err)
	}
	return true
}

func (db *DB) removeNoPersist(id string) bool {
	item, exists := db.items[id]
	if !exists {
		return false
	}
	delete(db.items, id)

	for k, v := range item.meta {
		if _, exists := db.revMap[k]; exists {
			if set, exists := db.revMap[k][v]; exists {
				set.Remove(id)
				if len(set.Items()) == 0 {
					delete(db.revMap[k], v)
				}
			}
			if len(db.revMap[k]) == 0 {
				delete(db.revMap, k)
			}
		}
	}
	return true
}

// update updates an existing item in the database. It returns false if the item does not exist.
func (db *DB) update(id string, meta MetaMap) bool {
	if !db.removeNoPersist(id) {
		return false
	}
	if !db.addNoPersist(id, meta) {
		return false
	}

	if err := db.saveLocked(); err != nil {
		log.Printf("bookindex: failed to persist update for %s: %v", id, err)
	}
	return true
}

// Query returns a list of item IDs that match the given metadata key and value.
func (db *DB) Query(metaKey, metaVal string) []string {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if _, exists := db.revMap[metaKey]; exists {
		if set, exists := db.revMap[metaKey][metaVal]; exists {
			return set.Items()
		}
	}
	return []string{}
}

func (db *DB) GetMetaGroup(metaKey string) map[string]*util.Set[string] {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if _, exists := db.revMap[metaKey]; !exists {
		return nil
	}

	// deep copy
	result := make(map[string]*util.Set[string])
	for k, v := range db.revMap[metaKey] {
		result[k] = v.Copy()
	}

	return result
}

func copyMeta(meta MetaMap) MetaMap {
	if meta == nil {
		return nil
	}

	out := make(MetaMap, len(meta))
	maps.Copy(out, meta)
	return out
}

func (db *DB) saveLocked() error {
	state := persistedState{Items: make(map[string]MetaMap, len(db.items))}
	for id, item := range db.items {
		state.Items[id] = copyMeta(item.meta)
	}

	bs, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return db.bdb.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(badgerStateKey), bs)
	})
}

func (db *DB) load() error {
	var state persistedState
	err := db.bdb.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(badgerStateKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &state)
		})
	})
	if err == badger.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	for id, meta := range state.Items {
		db.addNoPersist(id, meta)
	}

	return nil
}
