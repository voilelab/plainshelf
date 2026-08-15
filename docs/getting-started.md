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

Use the shelf controls in the app to add a local shelf directory.

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

!!! tip "Keep the server private"
    Bind the port to `127.0.0.1` unless PlainShelf is behind a trusted VPN or
    authentication boundary. The default Docker configuration does not enable
    application-level authentication.

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

On the Android app and narrow browser screens, the reader uses an immersive
layout: tap the center of the page to show or hide its controls, swipe left for
the next chapter, and swipe right for the previous chapter. Vertical swipes
continue to scroll within the current chapter.

An EPUB is converted to text as it is imported; see
[EPUB Import](epub-import.md) for what is kept, what is dropped, and how to
choose the output layout.

PlainShelf creates a `.bookpkg` directory in the shelf and assigns a stable book
ID. Renaming the title or moving the book between folders does not change that
ID. See [Data Model](concepts/data-model.md) for the on-disk layout.

## 4. Back up before experimenting

PlainShelf is pre-alpha. Back up both the configured shelf and application store
before upgrades or manual filesystem edits. Stop write activity first so the
backup captures a consistent state.

## Next steps

- [Installation and upgrades](installation.md)
- [Local shelf configuration](configuring-local-shelf.md)
- [SMB shelf configuration](configuring-smb-shelf.md)
- [Known issues](known-issue.md)
