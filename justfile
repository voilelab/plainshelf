set shell := ["zsh", "-cu"]

srv_frontend_dir := "frontend"
e2e_test_dir := "e2e"

# Version string injected into builds; derived from git, falls back to "dev".
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
version_pkg := "github.com/voilelab/plainshelf/internal/version"

default:
	just --list

# Build server: build frontendt
build-server-frontend:
	npm --prefix {{srv_frontend_dir}} run build

# Run tests: run Go tests for server and desktop.
test-go: build-server-frontend
	go test ./...
	cd desktop && go test ./...

# Run e2e tests: build frontend and run e2e tests.
test-e2e: build-server-frontend
	npm --prefix {{e2e_test_dir}} ci
	npx --prefix {{e2e_test_dir}} playwright install --with-deps chromium
	npm --prefix {{e2e_test_dir}} test

# Build server: build Go server binary.
build-server-backend: build-server-frontend
	go build -ldflags "-X {{version_pkg}}.Version={{version}}" -o plainshelf-srv cmd/plainshelf-srv/main.go

# Build desktop app
build-desktop: build-server-frontend
	cd desktop && go mod tidy && go tool wails build -ldflags "-X {{version_pkg}}.Version={{version}}"

# Run desktop app
run-desktop: build-server-frontend
	cd desktop && go mod tidy && go tool wails dev
