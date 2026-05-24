# Getting Started

This page walks you through the quickest way to run PlainShelf locally.

## Prerequisites

- [Go](https://go.dev/dl/) 1.21 or later
- [Node.js](https://nodejs.org/) 22 or later and npm
- Git

---

## Option 1 — Frontend only (mock data)

The fastest way to explore the UI without a running backend:

```bash
cd frontend
npm install

# Start dev server with mock data
VITE_USE_MOCK_API=true npm run dev
```

The frontend dev server starts at <http://localhost:5173> by default and uses built-in mock data so you can browse the UI without configuring a shelf.

---

## Option 2 — Full local server

### 1. Build the frontend

```bash
cd frontend
npm install
npm run build
cd ..
```

### 2. Create a workspace directory

```bash
mkdir workspace
cp cmd/plainshelf-srv/conf/config.yaml workspace/
```

### 3. Start the server

```bash
cd workspace
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

The server is now listening on <http://127.0.0.1:20000>.

!!! info "Default development config"
    - Listens on `127.0.0.1:20000`
    - Stores shelf and mark data under the current working directory
    - Enables `local_token` security for mutating `/api` requests

    The server generates an ephemeral token at startup, injects it into the
    served frontend, and accepts it via `X-PlainShelf-Token` or
    `Authorization: Bearer <token>`.

---

## Option 3 — Docker

See the [Docker](development/docker.md) page for container-based setup.

---

## Run tests

```bash
# Build the frontend first (required by Go embed)
npm --prefix frontend run build

go test ./...
```
