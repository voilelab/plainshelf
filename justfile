set shell := ["zsh", "-cu"]

gui_dir := "cmd/txtlib-gui"
srv_frontend_dir := "txtlib-frontend"

default:
	just --list

# Build/package a desktop app bundle for macOS.
fyne-macos:
	go tool fyne package --src {{gui_dir}} -os darwin

# Build/package an Android app.
fyne-android:
	cd {{gui_dir}} && go tool fyne package -os android && mv Txtlib.apk ../../

# Build server: build frontendt
server-frontend:
	npm --prefix {{srv_frontend_dir}} run build

# Build server: build Go server binary.
server-backend: server-frontend
	go build -o txtlib-srv cmd/txtlib-srv/main.go
