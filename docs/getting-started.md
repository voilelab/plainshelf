# Getting Started

This guide takes you from an installed PlainShelf release to your first local
library. To build the project itself, use [Local Development Setup](development/setup.md).

## 1. Choose how to run PlainShelf

### macOS desktop app

Install and open the Homebrew app:

```bash
brew install --cask voilelab/plainshelf/plainshelf
open -a PlainShelf
```

Use the shelf controls in the app to add a shelf. Creating one needs only a
name — PlainShelf makes the folder in its own shelves directory — or you can
choose an existing folder to open instead.

### Prebuilt server

After extracting a release archive, copy and edit the sample configuration:

```bash
cp config.sample.yaml config.yaml
./plainshelf-srv -conf config.yaml
```

Open <http://127.0.0.1:20000>. Relative paths in `config.yaml` are resolved from
the directory where the server starts; use absolute paths for service installs.

### Docker

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  ghcr.io/voilelab/plainshelf:latest
```

Open <http://127.0.0.1:20000>. The named volume preserves the shelf and
application store when the container is replaced.

!!! warning "Keep the server private"
    The default container config protects writes with `local_token`, which is a
    CSRF boundary and not a login: anything that can reach the port can read the
    token out of the served page. Keep the port published on `127.0.0.1`, as
    above, and put a real boundary (reverse proxy auth or a VPN) in front before
    exposing it — see [Deployment and threat
    model](deployment-and-threat-model.md). Opening the UI through any other
    origin needs that origin listed as well; the [Docker](development/docker.md)
    page has both settings.

## 2. Configure storage

A server configuration needs two durable locations:

- `app_conf.shelves[].lib_root` stores book packages and shelf runtime state.
- `app_conf.store_path` stores server settings; reading progress, history, and time stay on each client device.

The sample configuration works for local development. Before a long-running
deployment, review [Local Shelf File Source](configuring-local-shelf.md).

## 3. Add a book

Open the library, choose **Import**, and select a `.txt`, `.md` or `.epub` file.
You can then edit its metadata, add a cover, place it in a folder, and open the
reader.

A Markdown book's detail page also lists the chapters its source splits into,
the same H2 headings the reader navigates by. Selecting one opens the reader at
that chapter instead of your last position. Plain text has no chapters, so the
list is absent there.

On the Android app and narrow browser screens, the reader uses an immersive
layout: tap the center of the page to show or hide its controls, swipe left for
the next chapter, and swipe right for the previous chapter. Vertical swipes
continue to scroll within the current chapter. These gestures are shown once the
first time you open a book; the **?** button in the reader's controls brings the
reminder back at any time.

An EPUB is converted to text as it is imported; see
[EPUB Import](epub-import.md) for what is kept, what is dropped, and how to
choose the output layout.

PlainShelf creates a `.bookpkg` directory in the shelf and assigns a stable book
ID. Renaming the title or moving the book between folders does not change that
ID. See [Data Model](concepts/data-model.md) for the on-disk layout.

## 4. Back up before experimenting

PlainShelf is pre-alpha. Until it reaches 1.0.0, the on-disk format itself can
still change between releases, so upgrading a v0.x shelf may require a fresh
start for some data; see
[Compatibility policy](concepts/data-format-versioning.md#compatibility-policy).
Back up both the configured shelf and application store before upgrades or
manual filesystem edits. Stop write activity first so the backup captures a
consistent state.

## Next steps

- [Installation and upgrades](installation.md)
- [Local shelf configuration](configuring-local-shelf.md)
- [SMB shelf configuration](configuring-smb-shelf.md)
- [Known issues](known-issue.md)
