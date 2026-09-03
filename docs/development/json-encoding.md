# JSON Encoding

PlainShelf is moving from `encoding/json` to `encoding/json/v2`. This page is
what the conversion follows: which option set a call site marshals with, what
each v1 API becomes, and which v2 defaults are adopted on purpose.

Written for PSW-95. Until that epic finishes, both packages are present in the
repository and `internal/repocheck` tracks which files still hold the old one.

---

## Why the API changes at all

Go 1.27's `encoding/json` already runs on the v2 implementation, so switching
imports buys no performance. It buys a deadline instead.

The behaviors worth having — empty arrays instead of `null`, `&` and `<`
written as themselves, duplicate object members rejected — are v2 *defaults*,
and they change bytes. Changing bytes is cheap now and expensive after the
`1.0.0-rc` freeze, when every writer would have to carry v1-compatibility
options forever to avoid a format change. That end state is the worst one
available: the v2 API with v1 semantics, paying for the migration without
collecting anything for it.

So the conversion happens before the freeze, in one series, and each batch
takes the new defaults as it goes.

---

## The option sets

Every marshal goes through `internal/jsonopt`. It exports three write sets, all
carrying `json.Deterministic(true)`, and one read set:

| Set | Output | Use it for |
|---|---|---|
| `jsonopt.Disk()` | indented two spaces | files a reader can open and edit: `book.json`, a source's `meta.json`, trash metadata, the exported book cache |
| `jsonopt.DiskCompact()` | one line | the machine-only files under `app/`: the fingerprint cache, the scan cache, stored reading progress |
| `jsonopt.API()` | one line | HTTP response bodies |
| `jsonopt.Read()` | — | decoding a file that is already on disk |

All three modules — the root module, `desktop`, and `reader` — import it
directly; `internal/` under the root module path is visible to both nested
modules through their `replace` directives.

### Why determinism is not optional

v2 marshals map entries in unspecified order. v1 always sorted them, and two
mechanisms in the shelf are built on "same content, same bytes":

- `shelf/fingerprint/cache.go` compares the freshly encoded cache against the
  bytes already on disk and skips the write when they match. `cacheFile` holds
  two maps.
- `shelf/shelf_cache_export.go` writes the exported book cache only when
  `bookCacheDigest` changes. That digest is a hash of a payload containing
  `Books map[string]BookCacheEntry`.
- `shelf/scancache/cache.go` does the same for `app/scan-cache.json` through
  `scanCacheDigest`.

Omitting `Deterministic` breaks none of them loudly. It compiles, it round-trips,
and a single-run golden test still passes. What it does is make every scan
rewrite files that did not change — which on a shelf held on pCloud or SMB is a
re-upload per scan, forever, on someone else's machine.

That is why the option lives in one package and why the assertions below exist.

### Why the read set goes the other way

`jsonopt.Read()` is the one set that *refuses* v2 defaults. v1 matched member
names case-insensitively, kept the last of a duplicate pair and replaced invalid
UTF-8; every shelf written so far may hold all three, so it carries
`MatchCaseInsensitiveNames(true)`, `jsontext.AllowDuplicateNames(true)` and
`jsontext.AllowInvalidUTF8(true)` — plus `MatchCaseSensitiveDelimiter(true)`,
which is the one option in the set that comes from the *v1* package.

That last one is why `internal/jsonopt/jsonopt.go` is on the import allowlist,
and it is not a detail. `MatchCaseInsensitiveNames` alone is not v1's matching:
v2 folds away underscores and dashes as well as case, so a `book.json` holding a
stray `"schema-version"` — ignored by v1 — would start binding to
`schema_version`, and a book whose schema version reads as a future one refuses
every write. Loosening the reader is as much a behavior change as tightening it,
so `TestReadMatchesV1` asserts the set against the v1 package itself rather than
against a table of remembered behaviors.

Adopting the strict defaults on the read side is not a formatting change like
the ones on the write side. `setMeta` rewrites `book.json` whole, so a member
this build fails to match is a member the next save deletes: a hand-edited
`"Title"` would read as absent and then be gone. That decision is PSW-99's,
after PSW-93 makes unknown members survive a write — see the table below.

---

## v1 → v2 API mapping

Import v2 under the name the call site already uses. `internal/jsonopt` is the
one file that also imports v1, for the option constructor described above:

```go
import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
)
```

| `encoding/json` | `encoding/json/v2` |
|---|---|
| `json.Marshal(v)` | `json.Marshal(v, jsonopt.Disk())` — same name, options added |
| `json.Unmarshal(b, &v)` | `json.Unmarshal(b, &v, jsonopt.Read())` — same name, options added |
| `json.MarshalIndent(v, "", "  ")` | `json.Marshal(v, jsonopt.Disk())`, which carries `jsontext.WithIndent("  ")` |
| `json.NewEncoder(w).Encode(v)` | `json.MarshalWrite(w, v, jsonopt.API())` |
| `json.NewDecoder(r).Decode(&v)` | `json.UnmarshalRead(r, &v, jsonopt.Read())` |
| `json.RawMessage` | `jsontext.Value` |
| `json.Valid(b)` | `jsontext.Value(b).IsValid()` |
| `decoder.DisallowUnknownFields()` | `json.RejectUnknownMembers(true)` as an option |
| `json.Marshaler` / `json.Unmarshaler` | unchanged — v2 still honors the v1 interfaces, which is why `util.JSONTime` and `util.JSONDate` need no work |

Two shape differences to expect while converting:

- `MarshalWrite` writes no trailing newline, while `Encoder.Encode` did. A
  response body does not care; a file compared byte-for-byte does.
- `UnmarshalRead` consumes the whole reader and rejects trailing data, where
  `Decoder.Decode` stopped after one value. `shelf/shelf_config.go` relies on
  the old behavior to detect trailing junk in `shelf.json` and needs a
  `jsontext.Decoder` rather than a straight substitution.

---

## v2 defaults: adopted or overridden

`internal/jsonopt`'s write sets carry determinism and indentation and nothing
else, so every row below marked "adopt" is live on the write side simply by not
being overridden. The three read-side rows are the exception: `jsonopt.Read()`
overrides them back to v1 until PSW-99 takes the decision.

| v2 default | Effect here | Decision |
|---|---|---|
| Map entries in unspecified order | Defeats the three "unchanged, do not rewrite" checks above | **Override.** `Deterministic(true)` in all three sets |
| `nil` slice and map encode as `[]` and `{}` | Settles the `"authors": null` question PSW-35 left open; a dozen or so API fields stop returning `null`. The Android pCloud reader already accepts both | **Adopt** (PSW-97, PSW-98) |
| `<`, `>`, `&` are not escaped | Only golden fixtures change. An escape sequence in a file whose selling point is that a text editor shows what you typed is a defect | **Adopt** (PSW-97) |
| Object names match case-sensitively | A hand-edited `"Title"` stops being read, and the next write drops it. Only safe once unknown members are preserved rather than discarded | **Adopt, but coupled** — PSW-99, together with the `json:",unknown"` passthrough PSW-93 wants. Held at v1 by `jsonopt.Read()` until then |
| Duplicate object members rejected | Pure gain for a hand-editable format, provided the error names the file and the member | **Adopt** (PSW-99); `jsonopt.Read()` holds it at v1 meanwhile |
| Invalid UTF-8 rejected | Same | **Adopt** (PSW-99); `jsonopt.Read()` holds it at v1 meanwhile |
| `omitempty` means "would encode as null, empty string, empty object or empty array" | Every use is on a string, slice, map or pointer field; PSW-40 already moved the bool and int fields to `omitzero` | **Adopt.** No visible change |

Not done, deliberately:

- `GOEXPERIMENT=nojsonv2` — an escape hatch that upstream plans to remove.
- Carrying v1-compatibility options to preserve existing bytes, which is the
  outcome the whole series exists to avoid.
- `jsontext` streaming decode for large `book-cache-*.json` files, and
  rewriting `util.JSONTime`/`JSONDate` as `MarshalerTo`/`UnmarshalerFrom`.
  Both are real, neither is urgent; open a ticket when there is a measurement.

---

## What the tests enforce

**Determinism, once.** `TestOptionSetsSortMapKeys` in `internal/jsonopt`
marshals a payload shaped like the ones the shelf writes — maps reached through
a struct field and through another map — 64 times with each exported set and
fails if the bytes ever move. It is asserted here and not once per on-disk type
on purpose: `Deterministic` sorts every map in a value at every depth, so
repeating the assertion for `cacheFile`, `BookCacheFile` and the rest would
re-test the standard library rather than anything this repository decides.

What is worth asserting per payload is a different claim — that the *writer*
passes the option at all — and that belongs in the conversion tickets, against
the real write path, once a call site uses `jsonopt`.

**A control for that assertion.** `TestOptionsWithoutDeterministicVaryTheOrder`
marshals the same payload *without* the option and fails if the order never
changes. Without it the assertion above would keep passing on a toolchain that
happened to sort maps, and would stop meaning anything.

**The import ban.** `internal/repocheck` fails on any Go file importing
`encoding/json` that `jsonV1Allowlist` does not excuse. Each entry names the
ticket that removes it, a second test fails when an entry outlives the import it
covers, and PSW-100 empties the map — after which the check is what keeps v1
out. The list covers all three modules and runs under `go test ./...`, which CI
already gates.

---

## Left for the conversion tickets

- `.golangci.yml` excludes `(*encoding/json.Encoder).Encode` from `errcheck`.
  The v2 equivalent is `json.MarshalWrite`, whose error should be handled rather
  than excluded; drop the exclusion when the last `Encoder` goes.
- The `json:"…"` struct tags need no edits. v2 reads the same tag syntax,
  including `omitempty` and `omitzero`.
