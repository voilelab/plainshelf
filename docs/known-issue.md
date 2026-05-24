# Known Issues

This page documents known limitations and issues in the current pre-alpha release of PlainShelf.

!!! warning "Pre-alpha"
    PlainShelf is in early development. APIs, data layout, and UI behavior may still change between releases.

---

## Pagination

Server-side pagination is not implemented yet. The frontend paginates the book list client-side, which means **all books are loaded into memory** on the first request regardless of library size.

---

## Supported formats

PlainShelf is TXT-focused. The following formats are outside the current scope and are not planned for v1:

- EPUB
- PDF
- CBZ / CBR
- DRM formats

---

## No multi-user support

PlainShelf is a single-user local application. There is no authentication system beyond the `local_token` mode used to protect mutating API requests on the local machine.

---

## Desktop client is experimental

The Wails-based desktop client is experimental and not part of the primary development focus. Expect rough edges while core `shelf` and `server` behavior is still evolving.

---

## Reporting new issues

Please report bugs and unexpected behavior on the [GitHub issue tracker](https://github.com/voilelab/plainshelf/issues).
