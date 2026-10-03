//go:build !(js && wasm)

package store

import badger "github.com/dgraph-io/badger/v4"

func storeOptions(dbPath string) badger.Options {
	return badger.DefaultOptions(dbPath)
}
