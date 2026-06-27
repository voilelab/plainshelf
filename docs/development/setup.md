# Local Development Setup

This page covers everything you need to build and run PlainShelf from source.

## Prerequisites

| Tool | Minimum version | Notes |
|---|---|---|
| Go | 1.26.1 | <https://go.dev/dl/> |
| Node.js | 22 | <https://nodejs.org/> |
| npm | bundled with Node.js | |
| just | any recent | <https://github.com/casey/just> — task runner used throughout the docs |

---

## Repository layout

```text
cmd/
└─ plainshelf-srv/  # server binary entrypoint

shelf/              # core library (Go)
server/             # HTTP server (Go)
frontend/           # Vue 3 web frontend
internal/           # shared internal utilities (Go)
desktop/            # Wails desktop client
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
# 1. Build the frontend (run `npm --prefix frontend install` once first)
just build-server-frontend

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
just test-go
```

End-to-end tests run via Playwright:

```bash
just test-e2e
```

---

## Desktop app

The desktop client uses [Wails](https://wails.io/).

Run it in development mode:

```bash
just run-desktop
```

Produce a release build:

```bash
just build-desktop
```

---

## Code style

- Go: follow standard `gofmt` formatting.
- TypeScript/Vue: the project uses Vite + `vue-tsc` for type checking. Run `npm run build` in `frontend/` to validate types.
