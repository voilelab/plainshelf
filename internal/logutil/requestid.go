package logutil

import (
	"context"
	"crypto/rand"
)

const requestIDAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

const requestIDLength = 8

func NewRequestID() string {
	const rejectAtOrAbove = byte(256 - 256%len(requestIDAlphabet))

	id := make([]byte, 0, requestIDLength)
	buf := make([]byte, requestIDLength)
	for len(id) < requestIDLength {
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if b >= rejectAtOrAbove {
				continue
			}
			id = append(id, requestIDAlphabet[int(b)%len(requestIDAlphabet)])
			if len(id) == requestIDLength {
				break
			}
		}
	}
	return string(id)
}

type requestIDContextKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func RequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}
