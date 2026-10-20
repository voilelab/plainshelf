package server

import (
	"net/http"
	"strings"
)

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

func (c *apiCore) cacheVisibility(r *http.Request) string {
	if c.security.requiresToken(r) {
		return "private"
	}
	return "public"
}

type imageCacheFreshness string

const (
	cacheUntilChanged imageCacheFreshness = "max-age=86400"

	cacheRevalidateAlways imageCacheFreshness = "no-cache"
)

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

func opaqueETag(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "W/")
}
