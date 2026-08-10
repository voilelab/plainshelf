# Installation

This page covers installing **prebuilt** PlainShelf releases. If you would
rather build from source, see [Local Development Setup](development/setup.md).

Every tagged release publishes:

- A Homebrew cask for the macOS desktop client
- Prebuilt server archives for Linux and macOS
- A multi-architecture Docker image on the GitHub Container Registry (GHCR)

Release artifacts live on the
[GitHub Releases](https://github.com/voilelab/plainshelf/releases) page.

!!! warning "Pre-alpha"
    PlainShelf is in early development. Pin to a specific release tag and
    expect data layout and behavior to change between versions.

---

## Option 1 — Homebrew desktop app (macOS, Apple Silicon)

The quickest way to install the desktop app on Apple Silicon Macs is with
Homebrew:

```bash
brew install --cask voilelab/plainshelf/plainshelf
```

If you prefer to tap the repository explicitly first:

```bash
brew tap voilelab/plainshelf https://github.com/voilelab/plainshelf
brew install --cask plainshelf
```

Upgrade with `brew upgrade --cask plainshelf`, uninstall with
`brew uninstall --cask plainshelf`.

The bundled `.app` is unsigned and unnotarized; the cask's `postflight`
clears Gatekeeper's quarantine attribute so the app opens normally on
first launch.

For other platforms, build the desktop client from source — see
[Local Setup](development/setup.md).

---

## Option 2 — Prebuilt server binary

### 1. Download

Grab the archive for your platform from the
[latest release](https://github.com/voilelab/plainshelf/releases/latest).
Archives are named:

```text
plainshelf_<version>_<os>_<arch>.tar.gz
```

Available builds:

| OS      | Architectures    |
| ------- | ---------------- |
| Linux   | `amd64`, `arm64` |
| macOS   | `arm64`          |

### 2. Verify the download (recommended)

Each release ships a `SHA256SUMS` file. After downloading both the archive
and `SHA256SUMS` into the same directory:

```bash
sha256sum --ignore-missing -c SHA256SUMS
```

### 3. Extract

```bash
mkdir -p plainshelf
tar -xzf plainshelf_<version>_linux_amd64.tar.gz -C plainshelf
cd plainshelf
```

Each archive contains the `plainshelf-srv` binary, `LICENSE`, `README.md`, the
version-matched `docs/` directory, the README preview image, and a
`config.sample.yaml` to use as a starting point. The relative documentation
links in `README.md` therefore work both online and from the extracted archive.

!!! note "macOS Gatekeeper"
    The macOS binaries are unsigned. On first run you may need to clear the
    quarantine attribute:

    ```bash
    xattr -d com.apple.quarantine ./plainshelf-srv
    ```

### 4. Configure and run

```bash
cp config.sample.yaml config.yaml
./plainshelf-srv -conf config.yaml
```

By default the server listens on <http://127.0.0.1:20000>. See
[Local Shelf File Source](configuring-local-shelf.md) for shelf configuration.

---

## Option 3 — Docker

Tagged releases push a multi-arch (`linux/amd64`, `linux/arm64`) image to
GHCR at `ghcr.io/voilelab/plainshelf`.

```bash
# Pin to a release tag
docker pull ghcr.io/voilelab/plainshelf:v1.0.0

# Or track the most recent stable release
docker pull ghcr.io/voilelab/plainshelf:latest
```

!!! info "`latest` tag"
    `latest` only follows stable releases. Pre-release tags (those containing
    a `-`, e.g. `v1.0.0-rc.1`) are published but do **not** update `latest`.

Run the server on <http://localhost:20000> with data persisted in a volume:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  ghcr.io/voilelab/plainshelf:latest
```

!!! tip "Keep it local"
    The example publishes the port on the loopback address (`127.0.0.1`)
    only. Do not expose `0.0.0.0:20000` to untrusted networks unless you add
    an authentication boundary in front of the container.

For custom configuration and the bundled defaults, see the
[Docker](development/docker.md) page.

---

## Upgrading

!!! warning "v0.8 reading history and reading time do not carry into v1"
    v1 deliberately starts new per-device records. v0.8's server-side values
    are not migrated and no longer appear after upgrading. PlainShelf provides
    no export, import, or recovery path for them. See
    [v0.8 reading-data breaking change](concepts/data-format-versioning.md#v08-reading-data-breaking-change)
    for details.

1. Stop the running server (or `docker stop plainshelf`).
2. Download/pull the new version using the steps above.
3. Restart against the **same** data and config.

Because PlainShelf keeps data in human-readable files, back up your shelf and
application store directories before upgrading across breaking changes.
