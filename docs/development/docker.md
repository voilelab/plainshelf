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

!!! warning "Keep it local — `local_token` is not a boundary for an exposed port"
    Publish the port on the loopback address (`127.0.0.1`) only. `local_token`
    mode rejects cross-origin (CSRF) writes and lets the browser authenticate
    with no manual setup, but it is **not** an authentication boundary for an
    exposed port: any client that can reach the port may `GET /` and read the
    token straight out of the served page, then write. Do not expose
    `0.0.0.0:20000` to untrusted networks unless you put a real boundary
    (reverse proxy auth or a VPN edge) in front of the container.

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
- Sets `app_conf.security.mode: "local_token"`, so `/api` requests that write
  (`POST`/`PUT`/`PATCH`/`DELETE`, except the shelf rescan, which writes nothing)
  require a token. The server injects that token into the served index page, so a
  browser opened against the container needs no manual setup.

`local_token` allows only loopback browser origins on port 20000 by default
(`http://127.0.0.1:20000`, `http://localhost:20000`). If you open the UI from a
different host port (for example `-p 127.0.0.1:8080:20000`), a LAN IP (for
example a NAS), or a custom domain, add that exact origin to
`app_conf.security.allowed_origins` in a mounted config or the Origin check will
reject writes. Switch back to `mode: "none"` only when an authentication
boundary (reverse proxy auth or a VPN edge) already sits in front of the
container.

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
