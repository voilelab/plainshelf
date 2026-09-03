// Package jsonopt holds the encoding/json/v2 option sets PlainShelf encodes and
// decodes with, so one decision covers every call site instead of each one
// remembering the same options.
//
// The option that motivated the package is [json.Deterministic]. v2 marshals
// map entries in unspecified order, while v1 always sorted them, and two
// PlainShelf mechanisms are built on "same content, same bytes": the fingerprint
// cache compares the encoded file against the one on disk before rewriting it,
// and the exported book cache is keyed by a digest of its own payload. Dropping
// the option costs no compile error and no test failure — only a pointless
// upload on every scan for a shelf held on pCloud or SMB.
//
// Everything else on the write side is a deliberate non-decision: those sets
// add nothing but determinism and indentation, so the remaining v2 defaults
// (empty slices and maps as [] and {}, no HTML escaping) apply as they are.
// [Read] is the exception, and overrides in the opposite direction: it holds
// the decoder at v1's tolerance until the shelf can afford to tighten it.
// docs/development/json-encoding.md records why each default is accepted.
package jsonopt

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
)

// diskIndent is the indentation every hand-editable file on the shelf is
// written with. It is two spaces because that is what the shelf already
// contains, and rewriting every book.json to change it would be a data-format
// change for no benefit.
const diskIndent = "  "

var (
	disk = jsonv2.JoinOptions(
		jsonv2.Deterministic(true),
		jsontext.WithIndent(diskIndent),
	)

	diskCompact = jsonv2.JoinOptions(
		jsonv2.Deterministic(true),
	)

	api = jsonv2.JoinOptions(
		jsonv2.Deterministic(true),
	)

	read = jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
	)
)

// Disk returns the options for a file a human is meant to be able to open and
// edit: book.json, a source's meta.json, trash metadata, the exported book
// cache. Output is indented and map keys are sorted.
func Disk() jsonv2.Options { return disk }

// DiskCompact returns the options for the machine-only files under app/ that
// are written on a single line — the fingerprint cache, the scan cache, stored
// reading progress. It is [Disk] without the indentation, not a weaker
// guarantee: these are exactly the files whose bytes are compared or digested,
// so the determinism matters more here than anywhere else.
func DiskCompact() jsonv2.Options { return diskCompact }

// API returns the options for an HTTP response body. Responses are unindented
// to keep them small, and deterministic so that a body is worth comparing —
// in a contract test, in a diff between two builds, or behind an ETag.
func API() jsonv2.Options { return api }

// Read returns the options for decoding a file that is already on disk. Unlike
// the write sets it exists to *refuse* v2 defaults: v1 matched member names
// case-insensitively, kept the last of a duplicate pair and replaced invalid
// UTF-8, and a shelf written by any build up to now may hold all three.
//
// Adopting the strict defaults here is not a formatting change like the ones on
// the write side, because a member this build fails to match is a member the
// next whole-file write drops — a hand-edited "Title" would be read as absent
// and then deleted. PSW-99 makes that decision once unknown members survive a
// write; until then the reader stays where it was.
func Read() jsonv2.Options { return read }
