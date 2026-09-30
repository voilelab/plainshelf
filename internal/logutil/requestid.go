package logutil

import (
	"context"
	"crypto/rand"
)

// requestIDAlphabet is Crockford base32 with "0" and "1" removed.
//
// Crockford already drops I, L, O and U. Dropping the two digits as well
// removes the last characters a person confuses when a request ID is read
// aloud or copied by hand off a screen - 0 against O, and 1 against I and L -
// which is the only reason the ID is shown to a user at all.
//
// The cost is that 30 symbols is not a power of two, so a requestIDLength
// string carries log2(30^8) = 39.2 bits rather than the 40 an unrestricted
// base32 alphabet would. That is a factor of 1.7 in collision resistance,
// against an ID that only has to stay unique inside one log retention window.
const requestIDAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// requestIDLength is short enough to dictate over the phone and long enough
// that two IDs in the same retention window do not collide.
const requestIDLength = 8

// NewRequestID returns a fresh request ID.
//
// It is the single generator for every ID a bug report can quote: the
// X-Request-Id header, the request log line, the error envelope's incident
// field and the background chains a request queues all take their ID from
// here, so a reported number always names exactly one request.
func NewRequestID() string {
	// Bytes at or above this bound are discarded rather than folded, which
	// would hand the first 256 % 30 symbols a higher share than the rest.
	const unbiasedBound = byte(256 - 256%len(requestIDAlphabet))

	id := make([]byte, 0, requestIDLength)
	buf := make([]byte, requestIDLength)
	for len(id) < requestIDLength {
		// crypto/rand.Read never reports an error; it panics if the system
		// source fails.
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if b >= unbiasedBound {
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

// WithRequestID carries a request's ID down to whatever answers it. The value
// travels in the context rather than in a parameter because the handlers that
// need it are several helper calls away from the middleware that minted it.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

// RequestIDFrom returns the request ID carried by ctx, or "" for a context
// that never passed through the middleware - a background job, or a handler
// called directly by the desktop client.
func RequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}
