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
    Publishing the port on the loopback address (`127.0.0.1`) is still
    recommended, but with the default `local_token` mode it is no longer the
    only line of defence: unauthenticated writes are rejected even if the port
    is reachable.

## Default container config

The image uses `docker/config.yaml`, which:

- Listens on `0.0.0.0:20000` inside the container
- Stores data in `/data/shelf` and `/data/store`
- Sets `app_conf.security.mode: "local_token"`, so mutating `/api` requests
  (`POST`/`PUT`/`PATCH`/`DELETE`) require a token. The server injects that token
  into the served index page, so a browser opened against the container needs no
  manual setup.

`local_token` allows only loopback browser origins by default
(`http://127.0.0.1:20000`, `http://localhost:20000`). If you open the UI from a
LAN IP (for example a NAS) or a custom domain, add those origins to
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
