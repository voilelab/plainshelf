# JSON Encoding

PlainShelf is moving from `encoding/json` to `encoding/json/v2`. This page is
what the conversion follows: which option set a call site marshals with, what
each v1 API becomes, and which v2 defaults are adopted on purpose.

Written for PSW-95. The conversion is enforced rather than agreed: importing
`encoding/json` fails `TestNoEncodingJSONV1Imports` in `internal/repocheck`
unless the file is named in that test's allowlist, which only shrinks. One file
is still on it — `shelf/shelf_config.go`, whose read path is PSW-99's — so check
which package a file imports before copying a call site from it.

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

Every marshal goes through `internal/jsonopt`. It exports three sets, all
carrying `json.Deterministic(true)`:

| Set | Output | Use it for |
|---|---|---|
| `jsonopt.Disk()` | indented two spaces | files a reader can open and edit: `book.json`, a source's `meta.json`, trash metadata, the exported book cache |
| `jsonopt.DiskCompact()` | one line | the machine-only files under `app/`: the fingerprint cache, the scan cache, stored reading progress, the values in the settings table |
| `jsonopt.API()` | one line | HTTP response bodies |

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

---

## v1 → v2 API mapping

The v2 package is itself named `json`, so it needs no import alias — a call
site keeps reading as `json.Marshal`:

```go
import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)
```

| `encoding/json` | `encoding/json/v2` |
|---|---|
| `json.Marshal(v)` | `json.Marshal(v, jsonopt.Disk())` — same name, options added |
| `json.Unmarshal(b, &v)` | unchanged |
| `json.MarshalIndent(v, "", "  ")` | `json.Marshal(v, jsonopt.Disk())`, which carries `jsontext.WithIndent("  ")` |
| `json.NewEncoder(w).Encode(v)` | `json.MarshalWrite(w, v, jsonopt.API())` |
| `json.NewDecoder(r).Decode(&v)` | `json.UnmarshalRead(r, &v)` |
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

`internal/jsonopt` sets determinism and indentation and nothing else, so every
row below marked "adopt" is live simply by not being overridden — on the read
side too: the shelf makes no compatibility promise about a file a v1 build
wrote and a v2 build reads.

| v2 default | Effect here | Decision |
|---|---|---|
| Map entries in unspecified order | Defeats the three "unchanged, do not rewrite" checks above | **Override.** `Deterministic(true)` in all three sets |
| `nil` slice and map encode as `[]` and `{}` | Settles the `"authors": null` question PSW-35 left open; a dozen or so API fields stop returning `null`. The Android pCloud reader already accepts both | **Adopt** (PSW-97, PSW-98). See [the API's array contract](#the-apis-array-contract) |
| `<`, `>`, `&` are not escaped | An escape sequence in a file whose selling point is that a text editor shows what you typed is a defect. On disk only golden fixtures change; the two places that inject JSON into an inline `<script>` escape `<` themselves (see below) | **Adopt** (PSW-97, PSW-100) |
| Object names match case-sensitively | A hand-edited `"Title"` stops being read, and `setMeta` rewrites the file whole, so the next save drops it. The unknown-member passthrough PSW-93 wants is what turns that into a preserved field | **Adopt** (PSW-97); reporting and passthrough are PSW-99's |
| Duplicate object members rejected | Pure gain for a hand-editable format, provided the error names the file and the member | **Adopt** (PSW-97); naming the file and member is PSW-99's |
| Invalid UTF-8 rejected | Same | **Adopt** (PSW-97) |
| `omitempty` means "would encode as null, empty string, empty object or empty array" | Every use is on a string, slice, map or pointer field; PSW-40 already moved the bool and int fields to `omitzero` | **Adopt.** No visible change |

Not done, deliberately:

- `GOEXPERIMENT=nojsonv2` — an escape hatch that upstream plans to remove.
- Carrying v1-compatibility options to preserve existing bytes, which is the
  outcome the whole series exists to avoid.
- `jsontext` streaming decode for large `book-cache-*.json` files, and
  rewriting `util.JSONTime`/`JSONDate` as `MarshalerTo`/`UnmarshalerFrom`.
  Both are real, neither is urgent; open a ticket when there is a measurement.

---

## The API's array contract

Every HTTP response body is marshalled by one of two writers — `writeJSON` in
`server/apicore.go` and `writeJSON` in `reader/readerapi/api.go` — and both go
through `jsonopt.API()`. An array-valued field therefore reaches the client as
`[]` when it is empty, and never as `null`. A client can index it without a
`?? []` guard of its own.

The desktop client is outside this: its Wails bindings return Go values that
Wails serializes with its own encoder, which this repository does not configure.

The exception is `omitempty`, which v2 defines as "would encode as null, empty
string, empty object or empty array": a field carrying it is *absent* when
empty rather than `[]`, so the guard is still needed there. Which fields carry
it is the whole of the difference:

| Field | Empty value on the wire |
|---|---|
| `authors`, `folder`, `succeeded_ids`, `failures`, `tasks`, `conflicting_book_ids` | `[]` |
| `tags`, `identifiers`, and the trash listing's `authors` and `original_folder` | absent |

`server/contract` pins both halves — `assertJSONArray` for the first row and
`TestAPIOmitEmptyMetaFieldsStayOmittedContract` for the second — because the
difference is invisible in Go, where a nil and an empty slice decode alike.

Request decoding is unaffected: a `*[]string` field left nil by an absent member
is also left nil by an explicit `null`, in v1 and v2 alike. Clearing a list
through PATCH is spelled `[]`, and `null` means "leave this alone".

---

## JSON inside an inline `<script>`

Two handlers write a marshalled payload straight into `index.html` inside a
`<script>` element: `injectSecurityBootstrap` in `server/spa.go`, which carries
the local token and the configured `token_header`, and `indexHTML` in
`reader/readerapi/spa.go`, which carries the open book's ID. Both replace `<`
with `\u003c` after marshalling, and that replacement is load-bearing: v1 escaped
`<` in every string it wrote, v2 does not, and `<` is the only character that can
close the element. `\u003c` survives `JSON.parse` unchanged, so nothing else has
to know.

`token_header` comes from the config file and a book ID is a directory name, so
neither is a value this repository chose. `TestInjectSecurityBootstrapEscapesScriptClose`
and `TestSPAEscapesScriptCloseInTheBootConfig` fail if the replacement is dropped.

---

## What the tests enforce

**Determinism, per write path.** Each of the three "unchanged content, do not
rewrite" checks has a case that writes the same data eight times over and fails
if the bytes or the file's mtime move: `TestFingerprintCacheWithManyEntriesIsByteStable`,
`TestBookCacheExportOfManyBooksSkipsUnchangedContent` and
`TestScanCacheWithManyDirectoriesIsNotRewritten`. Each uses twelve to sixteen
map entries, because a map with one entry has no order to vary and passes with
or without the option; removing `jsonopt.DiskCompact()` from one write path
fails that path's case and no other.

**The v1 import, repository-wide.** `internal/repocheck`'s
`TestNoEncodingJSONV1Imports` parses every `.go` file under the repository root —
all three modules, tests included, since a root `go test ./...` compiles neither
`desktop` nor `reader` — and fails on an `encoding/json` import that is not in
`encodingJSONAllowlist`. `TestEncodingJSONAllowlistIsCurrent` fails on an entry
whose file is gone or no longer imports v1, so converting a file also closes its
exemption rather than leaving a hole at that path.

---

## Left for the conversion tickets

- The `json:"…"` struct tags need no edits. v2 reads the same tag syntax,
  including `omitempty` and `omitzero`.
- `shelf/shelf_config.go` is the last v1 importer. Switching it adopts v2's
  strict member matching for `shelf.json`, which is a read-behavior decision
  rather than an import swap, so it belongs to PSW-99 along with the rest of the
  strict-read work. Emptying `encodingJSONAllowlist` is what finishes it.
