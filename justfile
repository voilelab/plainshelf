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
