//go:build js && wasm

package store

func lockStore(string) (func() error, error) {
	return func() error { return nil }, nil
}
