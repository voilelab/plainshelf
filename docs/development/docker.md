# Docker

PlainShelf ships an Ubuntu 24.04-based container image that bundles the server
and the embedded frontend.

## Build the image

From the repository root:

```bash
docker build -t plainshelf .
```

## Run the container

Start the server on <http://localhost:20000> with persistent application data
stored in a Docker volume:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  plainshelf
```

!!! tip "Keep it local"
    The example above publishes the port on the loopback address
    (`127.0.0.1`) only. Do not expose `0.0.0.0:20000` to untrusted networks
    unless you add an authentication boundary in front of the container.

!!! warning "`mode: none` on a non-loopback address"
    With `app_conf.security.mode: "none"`, the API answers every request —
    reads, writes, and deletes — without authentication. When the server is
    also bound to a non-loopback address, any device that can reach it has full
    access to your library. The Web UI shows a persistent, collapsible warning
    in this case so the exposure is not forgotten. It does not appear when the
    server is bound to loopback, which is the ordinary local-only setup.

## Default container config

The image uses `docker/config.yaml`, which:

- Listens on `0.0.0.0:20000` inside the container
- Stores data in `/data/shelf` and `/data/store`
- Sets `app_conf.security.mode: "none"` for compatibility with local-only port
  publishing

## Custom configuration

Mount your own config file over `/etc/plainshelf/config.yaml`:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  -v "$PWD/path/to/config.yaml:/etc/plainshelf/config.yaml:ro" \
  plainshelf
```

## Health check

The image exposes a `/health` endpoint. Docker uses it automatically once the
container starts; to check it by hand:

```bash
curl http://127.0.0.1:20000/health
```
