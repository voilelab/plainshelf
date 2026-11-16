# Getting started

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
name, because PlainShelf makes the folder in its own shelves directory, or you
can choose an existing folder to open instead.

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

Open the library, choose **Import**, and select a `.txt`, `.md`, `.epub` or `.cbz` file.
You can then edit its metadata, add a cover, place it in a folder, and open the
reader.

[Library and reader](library-and-reader.md) covers importing several files at
once, the book detail page, search, and the reader's touch gestures.

An EPUB is converted to text as it is imported; see
[EPUB import](epub-import.md) for what is kept, what is dropped, and how to
choose the output layout.

A CBZ is extracted into an ordered image source; see [CBZ import](cbz-import.md)
for supported images, metadata, and limits.

PlainShelf creates a `.bookpkg` directory in the shelf and assigns a stable book
ID. Renaming the title or moving the book between folders does not change that
ID. See [Data model](concepts/data-model.md) for the on-disk layout.

## 4. Back up before experimenting

Until `1.0.0-rc1` an upgrade can still change the on-disk format. Back up the
shelf and the application store before upgrading or editing files by hand;
[Backup and restore](backup-and-restore.md) has the steps.

## Next steps

- [Installation and upgrades](installation.md)
- [Library and reader](library-and-reader.md)
- [Local shelf configuration](configuring-local-shelf.md)
- [SMB shelf configuration](configuring-smb-shelf.md)
- [Known issues](known-issue.md)
