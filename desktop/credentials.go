package main

import (
	"context"
	"errors"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/zalando/go-keyring"
)

// keychainService names PlainShelf's items in the OS keychain.
const keychainService = "PlainShelf"

// hostCredentialPrefix is the credential scheme the desktop answers itself;
// its ID is the shelf entry ID. See docs/concepts/shelf-sources.md#credentials.
const hostCredentialPrefix = "host:"

// errSecretNotFound is a host: reference with nothing stored under it.
var errSecretNotFound = util.NewError("no secret is stored for this shelf")

// secretStore keeps one secret per shelf entry ID.
type secretStore interface {
	Get(id string) (string, error)
	Set(id, secret string) error
	Delete(id string) error
}

// osKeychain is the OS keychain: Keychain on macOS, Credential Manager on
// Windows, the Secret Service on Linux.
type osKeychain struct{}

func (osKeychain) Get(id string) (string, error) {
	secret, err := keyring.Get(keychainService, id)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errSecretNotFound
	}
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return secret, nil
}

func (osKeychain) Set(id, secret string) error {
	if err := keyring.Set(keychainService, id, secret); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (osKeychain) Delete(id string) error {
	err := keyring.Delete(keychainService, id)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return util.Errorf("%w", err)
	}
	return nil
}

// desktopCredentialResolver answers host: from the secret store and hands
// every other scheme to the server's default.
type desktopCredentialResolver struct {
	store secretStore
}

func (r desktopCredentialResolver) Resolve(ctx context.Context, ref string) (string, error) {
	id, ok := strings.CutPrefix(ref, hostCredentialPrefix)
	if !ok {
		return server.DefaultCredentialResolver{}.Resolve(ctx, ref)
	}
	secret, err := r.store.Get(id)
	if err != nil {
		return "", util.Errorf("credential %q: %w", ref, err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", util.Errorf("credential %q: %w", ref, errSecretNotFound)
	}
	return secret, nil
}
