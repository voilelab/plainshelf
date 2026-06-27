# Installation

This page covers installing **prebuilt** PlainShelf releases. If you would
rather build from source, see [Getting Started](getting-started.md).

Every tagged release publishes:

- Prebuilt server archives for Linux, macOS, and Windows
- A multi-architecture Docker image on the GitHub Container Registry (GHCR)

Release artifacts live on the
[GitHub Releases](https://github.com/voilelab/plainshelf/releases) page.

!!! warning "Pre-alpha"
    PlainShelf is in early development. Pin to a specific release tag and
    expect data layout and behavior to change between versions.

---

## Option 1 — Prebuilt server binary

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

Each archive contains the `plainshelf-srv` binary, `LICENSE`, `README.md`,
and a `config.sample.yaml` to use as a starting point.

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

## Option 2 — Docker

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

## Desktop client

The Wails-based desktop client is **experimental** and is not yet part of the
release artifacts. To try it, build it from source — see
[Local Setup](development/setup.md).

---

## Upgrading

1. Stop the running server (or `docker stop plainshelf`).
2. Download/pull the new version using the steps above.
3. Restart against the **same** data and config.

Because PlainShelf keeps data in human-readable files, back up your shelf and
application store directories before upgrading across breaking changes.
