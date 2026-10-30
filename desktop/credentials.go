package main

import (
	"context"
	"errors"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/zalando/go-keyring"
)

const keychainService = "PlainShelf"

const hostCredentialPrefix = "host:"

var errSecretNotFound = util.NewError("no secret is stored for this shelf")

type secretStore interface {
	Get(id string) (string, error)
	Set(id, secret string) error
	Delete(id string) error
}

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
