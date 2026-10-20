// Package jsonopt holds the encoding/json/v2 option sets PlainShelf marshals
// with, so one decision covers every writer.
//
// The reason is [json.Deterministic]: v2 marshals maps in unspecified order (v1
// sorted them), and two mechanisms rely on same content giving same bytes. The
// fingerprint cache compares the encoded file with the one on disk before
// rewriting it, and the exported book cache is keyed by a digest of its payload.
// Dropping the option breaks no build or test; it costs an upload per scan on
// pCloud or SMB. Otherwise the sets add only indentation and keep the v2 defaults;
// docs/development/json-encoding.md says why each one is accepted.
package jsonopt

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// diskIndent is the indentation every hand-editable file on the shelf is
// written with. It is two spaces because that is what the shelf already
// contains, and rewriting every book.json to change it would be a data-format
// change for no benefit.
const diskIndent = "  "

var (
	disk = json.JoinOptions(
		json.Deterministic(true),
		jsontext.WithIndent(diskIndent),
	)

	diskCompact = json.JoinOptions(
		json.Deterministic(true),
	)

	api = json.JoinOptions(
		json.Deterministic(true),
	)
)

// Disk returns the options for a file a human is meant to be able to open and
// edit: book.json, a source's meta.json, trash metadata, the exported book
// cache. Output is indented and map keys are sorted.
func Disk() json.Options { return disk }

// DiskCompact returns the options for the machine-only files under app/ that
// are written on a single line — the fingerprint cache, the scan cache, stored
// reading progress. It is [Disk] without the indentation, not a weaker
// guarantee: these are exactly the files whose bytes are compared or digested,
// so the determinism matters more here than anywhere else.
func DiskCompact() json.Options { return diskCompact }

// API returns the options for an HTTP response body. Responses are unindented
// to keep them small, and deterministic so that a body is worth comparing —
// in a contract test, in a diff between two builds, or behind an ETag.
func API() json.Options { return api }
