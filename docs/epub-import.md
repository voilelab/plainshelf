# EPUB Import

PlainShelf can import EPUB files. EPUB is an **import format, not a storage
format**: the text is extracted and stored as an ordinary book, exactly like a
`.txt` or `.md` import. Nothing on the shelf becomes a binary blob, and every
imported book stays readable in a text editor.

## What is kept

| From the EPUB | Where it lands |
|---|---|
| Reading-order text | `sources/{source-id}/source.txt` |
| Table of contents chapter names | Chapter headings in the text, plus the source's split configuration |
| `dc:title` | Book title |
| `dc:creator` | Authors |
| `dc:language` | Language |
| `dc:description` | Book comments, and optionally the start of the text |
| `dc:identifier` | Identifiers, keyed by scheme (for example `isbn`) |
| `dc:date` | Published date |
| Cover image | `cover.jpg` / `cover.png` / … in the book directory |

The book's own `dc:title` is preferred over the uploaded filename. If you type a
title in the import dialog, that wins.

## What is dropped

- **The original `.epub` file.** It is not stored anywhere. Keep your own copy if
  you want one.
- **Embedded illustrations.** A book directory has no place to put them, and no
  route serves arbitrary files from inside one. The import counts them and
  records the total on the imported source, so the loss stays visible after the
  fact — see [What is recorded](#what-is-recorded) below.
- **Ruby annotations** (`<rt>`/`<rp>`). The base text is kept; furigana is
  removed so it does not interleave into Japanese prose.
- **Links, footnotes, and page structure.**
- **Inline emphasis**, when the output is plain text. The Markdown layout keeps
  bold and italic.

Because the original is not retained, re-importing is the only way to pick up
improvements to the converter.

## What is recorded

An EPUB that carried illustrations beyond its cover gets a note on the source
the import created, in `sources/{source-id}/meta.json`:

```json
{
  "comment": "Converted from EPUB. 2 embedded images were dropped."
}
```

The book detail view shows it as **Import notes**. Books that lost nothing get
no note at all, so the row only appears when there is something to report.

The cover is not counted: it is stored as the book's cover rather than dropped.
Images referenced more than once count once, so the number reflects distinct
artwork rather than the number of tags removed. Re-importing the same file
rewrites the note.

## Choosing the layout

The import dialog offers two layouts once an EPUB is selected. The choice
decides both how the text is written and the format the book is stored as.

### Markdown (default)

```markdown
# Book Title

The book description.

## Setting Out

The first chapter.
```

Stored with format `md`, so the reader renders it as Markdown. Chapter headings
carry a `## ` marker, which the source's split configuration matches as a regex —
so the reader's chapter list shows the **real chapter names**.

### Plain text

```text
Book Title

Setting Out

The first chapter.
```

Stored with format `txt`. There is no marker on a bare title line, so nothing can
distinguish a chapter heading from ordinary prose. The split falls back to
explicit line boundaries, which are exact but unnamed: the reader lists sections
as **"Part 1", "Part 2"** and so on. The chapter names are still present in the
text itself.

If the chapter list matters to you, use the Markdown layout.

### Description in the text

The book description is always saved to the book's metadata. This option
additionally writes it at the start of the text, before the first chapter.

## Setting the default

Import options chosen in the dialog apply to that batch only. To change the
default, open **Settings → Import**.

The default also governs imports that never open the dialog — notably the
desktop app's file picker, which imports immediately.

The default can be seeded from the server config file:

```yaml
epub_import_strategy:
  preset: markdown        # or: plain
  include_description: true
```

A value saved from the settings page takes precedence over the config file.
Deleting the saved value reverts to the config file, and then to the built-in
default (`markdown`, description included).

## Limits

- An import request is capped at 100 MB.
- A single document inside the archive is capped at 32 MB, and the whole book's
  extracted text at 256 MB.
- DRM-protected EPUB files are not supported and are not decrypted.
