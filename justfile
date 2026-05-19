set shell := ["zsh", "-cu"]

srv_frontend_dir := "frontend"

default:
	just --list

test:
    npm --prefix {{srv_frontend_dir}} run build
    go test ./...

# Build server: build frontendt
server-frontend:
	npm --prefix {{srv_frontend_dir}} run build

# Build server: build Go server binary.
server-backend: server-frontend
	go build -o plainshelf-srv cmd/plainshelf-srv/main.go

# Run Electron desktop shell (all platforms, dev mode)
gui-dev:
	npm --prefix {{srv_frontend_dir}} run electron:dev

# Build macOS .app via electron-builder (requires macOS host).
# Produces unpacked app bundle only (no dmg/zip signing pipeline).
macos-app:
	if [[ "$(uname -s)" != "Darwin" ]]; then \
		echo "macos-app recipe must run on macOS" >&2; \
		exit 1; \
	fi
	npm --prefix {{srv_frontend_dir}} run build
	npm --prefix {{srv_frontend_dir}} run sidecar:build
	cd {{srv_frontend_dir}} && npx --yes electron-builder@^26 --mac dir
