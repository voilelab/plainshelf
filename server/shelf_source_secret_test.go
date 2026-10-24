package server

import (
	"context"
	"errors"
	"testing"
)

type rotatingResolver struct {
	values []string
	calls  int
}

func (r *rotatingResolver) Resolve(context.Context, string) (string, error) {
	if len(r.values) == 0 {
		return "", errors.New("no secret")
	}
	value := r.values[min(r.calls, len(r.values)-1)]
	r.calls++
	return value, nil
}

func TestResolvableSecretRefusesAnUnresolvableRef(t *testing.T) {
	if _, err := resolvableSecret(&rotatingResolver{}, "env:MISSING"); err == nil {
		t.Fatal("resolvableSecret accepted a reference that does not resolve")
	}
}

func TestResolvableSecretResolvesAgainOnEveryCall(t *testing.T) {
	resolver := &rotatingResolver{values: []string{"first", "second", "third"}}
	secret, err := resolvableSecret(resolver, "env:TOKEN")
	if err != nil {
		t.Fatalf("resolvableSecret: %v", err)
	}
	for _, want := range []string{"second", "third"} {
		got, err := secret(t.Context())
		if err != nil || got != want {
			t.Fatalf("secret() = %q, %v; want %q", got, err, want)
		}
	}
}
