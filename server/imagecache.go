package server

import (
	"net/http"
	"strings"
)

// imageContentTypeForExt maps a stored image's file extension to the content
// type the read path serves it with. An unrecognized extension falls back to
// JPEG, which is what the cover path has always answered; source assets never
// reach the fallback because their names are validated first.
func imageContentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

// cacheVisibility is derived from the token gate itself rather than read from
// the config, so the two cannot drift apart. A gated response must not be stored
// by a shared cache: the token travels in a header the cache does not key on, so
// a stored copy could answer a request that never reached the gate.
func (c *apiCore) cacheVisibility(r *http.Request) string {
	if c.security.requiresToken(r) {
		return "private"
	}
	return "public"
}

// imageCacheFreshness is how long a client may reuse a stored image before it
// has to ask about it again.
type imageCacheFreshness string

const (
	// cacheUntilChanged suits a file whose URL changes when the file does. A
	// cover upload appends a cache-busting key (see getBookCoverUrl), so a
	// stale copy cannot outlive the change that made it stale.
	cacheUntilChanged imageCacheFreshness = "max-age=86400"

	// cacheRevalidateAlways suits a file the API can replace or remove while its
	// URL stays the same, which is every source asset: the reader derives the URL
	// from the file name in the text, and nothing records a version to bust a
	// cache with. Revalidation is cheap because the response carries an ETag and
	// answers 304 from a single stat.
	cacheRevalidateAlways imageCacheFreshness = "no-cache"
)

// serveImageValidator writes the caching headers for a stored image and reports
// whether it already answered the request with 304.
//
// An empty etag means the file could not be stat'd; the response then carries
// no validator and the caller goes on to serve the bytes.
func (c *apiCore) serveImageValidator(
	w http.ResponseWriter,
	r *http.Request,
	etag string,
	freshness imageCacheFreshness,
) bool {
	if etag == "" {
		return false
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", c.cacheVisibility(r)+", "+string(freshness))
	if etagMatchesIfNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	return false
}

// etagMatchesIfNoneMatch follows RFC 9110, which allows a list of validators or
// "*" and requires the weak comparison for GET and HEAD. Plain string equality
// answers a browser echoing one tag back but misses a client sending several,
// and the cost of a miss is a full body, unbounded for an asset.
//
// Splitting on commas is safe for the tags this server issues: they are
// mtime-size pairs and never contain one.
func etagMatchesIfNoneMatch(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" || etag == "" {
		return false
	}
	if header == "*" {
		return true
	}

	want := opaqueETag(etag)
	for candidate := range strings.SplitSeq(header, ",") {
		if opaqueETag(candidate) == want {
			return true
		}
	}

	return false
}

// opaqueETag drops the weak-validator prefix and surrounding space, which is
// what the weak comparison ignores.
func opaqueETag(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "W/")
}
