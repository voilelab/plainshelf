//go:build js && wasm

package store

import badger "github.com/dgraph-io/badger/v4"

// storeOptions keeps the store in memory: badger mmaps its files, which js/wasm cannot.
func storeOptions(string) badger.Options {
	return badger.DefaultOptions("").WithInMemory(true)
}
