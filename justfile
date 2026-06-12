set shell := ["zsh", "-cu"]

srv_frontend_dir := "frontend"
e2e_test_dir := "e2e"

default:
	just --list

# Build server: build frontendt
server-frontend:
	npm --prefix {{srv_frontend_dir}} run build

# Run tests: run Go tests for server and desktop.
test: server-frontend
	go test ./...
	cd desktop && go test ./...

# Run e2e tests: build frontend and run e2e tests.
e2e: server-frontend
	npm --prefix {{e2e_test_dir}} ci
	npx --prefix {{e2e_test_dir}} playwright install --with-deps chromium
	npm --prefix {{e2e_test_dir}} test

# Build server: build Go server binary.
server-backend: server-frontend
	go build -o plainshelf-srv cmd/plainshelf-srv/main.go

# Build desktop app
desktop: server-frontend
	cd desktop && go mod tidy && go tool wails build

# Build Android APK
android:
	npm --prefix {{srv_frontend_dir}} run cap:sync:android
	JAVA_HOME=/opt/homebrew/Cellar/openjdk@21/21.0.11/libexec/openjdk.jdk/Contents/Home gradle -p {{srv_frontend_dir}}/android assembleDebug
