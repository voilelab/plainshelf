# Local Development Setup

This page covers everything you need to build and run PlainShelf from source.

## Prerequisites

| Tool | Minimum version | Notes |
|---|---|---|
| Go | 1.21 | <https://go.dev/dl/> |
| Node.js | 22 | <https://nodejs.org/> |
| npm | bundled with Node.js | |

---

## Repository layout

```text
cmd/
└─ plainshelf-srv/  # server binary entrypoint

shelf/              # core library (Go)
server/             # HTTP server (Go)
frontend/           # Vue 3 web frontend
internal/           # shared internal utilities (Go)
desktop/            # experimental Wails desktop client
migration/          # shelf data migrations
```

---

## Frontend

### Development server (mock data)

```bash
cd frontend
npm install
VITE_USE_MOCK_API=true npm run dev
```

This starts Vite's hot-reload dev server at <http://localhost:5173> using built-in mock API responses — no backend required.

### Production build

```bash
cd frontend
npm install
npm run build
```

The compiled output lands in `frontend/dist/` and is embedded into the Go binary by `frontend/web.go`.

---

## Backend (Go server)

The Go server embeds the compiled frontend at build time, so the frontend must be built before `go build` or `go test` will succeed.

### Run the server

```bash
# 1. Build the frontend
npm --prefix frontend run build

# 2. Create a workspace
mkdir workspace
cp cmd/plainshelf-srv/conf/config.yaml workspace/

# 3. Start the server
cd workspace
go run ../cmd/plainshelf-srv/main.go -conf config.yaml
```

The server is available at <http://127.0.0.1:20000>.

### Run tests

```bash
npm --prefix frontend run build
go test ./...
```

---

## Desktop app (experimental)

The desktop client uses [Wails](https://wails.io/) and is currently experimental.

```bash
npm --prefix frontend run build
cd desktop
wails dev
```

!!! warning
    Expect rough edges while core shelf/server behavior is still evolving.

---

## Code style

- Go: follow standard `gofmt` formatting.
- TypeScript/Vue: the project uses Vite + `vue-tsc` for type checking. Run `npm run build` in `frontend/` to validate types.
