set shell := ["zsh", "-cu"]

srv_frontend_dir := "frontend"
e2e_test_dir := "e2e"

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
	go build -o plainshelf-srv cmd/plainshelf-srv/main.go

# Build desktop app
build-desktop: build-server-frontend
	cd desktop && go mod tidy && go tool wails build

# Run desktop app
run-desktop: build-server-frontend
	cd desktop && go mod tidy && go tool wails dev
