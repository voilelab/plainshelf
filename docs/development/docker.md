# Docker

PlainShelf ships an Ubuntu 24.04-based container image that bundles the server and the embedded frontend.

## Build the image

Run the following from the repository root:

```bash
docker build -t plainshelf .
```

---

## Run the container

Start the server on <http://localhost:20000> with persistent application data stored in a Docker volume:

```bash
docker run --rm \
  --name plainshelf \
  -p 127.0.0.1:20000:20000 \
  -v plainshelf-data:/data \
  plainshelf
```

!!! tip "Keep it local"
    The example above publishes the port on the loopback address (`127.0.0.1`) only. Do not expose `0.0.0.0:20000` to untrusted networks unless you add an authentication boundary in front of the container.

---

## Default container config

The image uses `docker/config.yaml`, which:

- Listens on `0.0.0.0:20000` inside the container
- Stores data in `/data/shelf` and `/data/store`
- Sets `app_conf.security.mode: "none"` for compatibility with local-only port publishing

---

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

---

## Health check

The image exposes a `/health` endpoint. Docker will use it automatically once the container starts. You can also test it manually:

```bash
curl http://127.0.0.1:20000/health
```
