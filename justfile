set shell := ["zsh", "-cu"]

srv_frontend_dir := "frontend"
e2e_test_dir := "e2e"

# Product version strings derived from Git tags for application builds.
version := `./scripts/resolve-version.sh display`
native_version := `./scripts/resolve-version.sh native`
android_version_name := `./scripts/resolve-version.sh android-name`
android_version_code := `count=$(git rev-list --count HEAD 2>/dev/null || echo 1); if [[ "$count" -gt 0 ]]; then echo "$count"; else echo 1; fi`
version_pkg := "github.com/voilelab/plainshelf/internal/version"

default:
	just --list

# Build server: build frontendt
build-server-frontend:
	npm --prefix {{srv_frontend_dir}} run build

# Run tests: run Go tests for server, desktop and reader.
test-go: build-server-frontend
	go test ./...
	cd desktop && go test ./...
	cd reader && go test ./...

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
	/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString {{native_version}}" desktop/build/bin/PlainShelf.app/Contents/Info.plist
	/usr/libexec/PlistBuddy -c "Set :CFBundleVersion {{native_version}}" desktop/build/bin/PlainShelf.app/Contents/Info.plist
	plutil -lint desktop/build/bin/PlainShelf.app/Contents/Info.plist
	test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' desktop/build/bin/PlainShelf.app/Contents/Info.plist)" = "{{native_version}}"
	test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' desktop/build/bin/PlainShelf.app/Contents/Info.plist)" = "{{native_version}}"

# Run desktop app
run-desktop: build-server-frontend
	cd desktop && go mod tidy && go tool wails dev

# Build reader app (standalone .bookpkg reader)
build-reader: build-server-frontend
	cd reader && go mod tidy && go tool wails build -ldflags "-X {{version_pkg}}.Version={{version}}"
	/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString {{native_version}}" reader/build/bin/PlainShelfReader.app/Contents/Info.plist
	/usr/libexec/PlistBuddy -c "Set :CFBundleVersion {{native_version}}" reader/build/bin/PlainShelfReader.app/Contents/Info.plist
	plutil -lint reader/build/bin/PlainShelfReader.app/Contents/Info.plist
	test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' reader/build/bin/PlainShelfReader.app/Contents/Info.plist)" = "{{native_version}}"
	test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' reader/build/bin/PlainShelfReader.app/Contents/Info.plist)" = "{{native_version}}"

# Run reader app in dev mode. Opens a book straight away when given one:
# `just run-reader path/to/book.bookpkg`; otherwise the app asks for a folder.
run-reader book="": build-server-frontend
	#!/usr/bin/env zsh
	set -eu
	cd reader
	go mod tidy
	if [[ -n "{{book}}" ]]; then
		go tool wails dev -appargs "-book {{book}}"
	else
		go tool wails dev
	fi

# Add the Android platform (one-time; requires Android SDK + JDK 21)
mobile-add-android: build-server-frontend
	cd {{srv_frontend_dir}} && npx cap add android

# Sync the web build and native plugins into the Android project
mobile-sync: build-server-frontend
	cd {{srv_frontend_dir}} && npx cap sync android

# Build a debug APK (run mobile-add-android first if android/ is missing)
build-mobile-android: mobile-sync
	#!/usr/bin/env zsh
	set -eu
	java_home="$(./scripts/resolve-android-jdk.sh)"
	cd {{srv_frontend_dir}}/android
	JAVA_HOME="$java_home" PLAINSHELF_VERSION_NAME="{{android_version_name}}" PLAINSHELF_VERSION_CODE="{{android_version_code}}" ./gradlew assembleDebug

# Open the Android project in Android Studio
open-mobile-android: mobile-sync
	cd {{srv_frontend_dir}} && npx cap open android

# Run the app on a local Android emulator with a local server (in-app server URL: http://10.0.2.2:20000)
run-android-app conf="config.yaml": build-server-frontend
	#!/usr/bin/env zsh
	set -eu
	java_home="$(./scripts/resolve-android-jdk.sh)"
	export JAVA_HOME="$java_home"
	if lsof -nP -iTCP:20000 -sTCP:LISTEN >/dev/null 2>&1; then
		echo "port 20000 is already in use — stop the running server first"; exit 1
	fi
	# Start the local server in the background; killed on exit/Ctrl-C.
	# Runs from the repo root, so config paths resolve relative to it (workspace/... style).
	[[ -f workspace/config.yaml ]] || { mkdir -p workspace && cp cmd/plainshelf-srv/conf/config.yaml workspace/; }
	( exec go run ./cmd/plainshelf-srv/main.go -conf workspace/{{conf}} ) &
	srv_pid=$!
	trap 'kill $srv_pid 2>/dev/null' EXIT INT TERM
	# Fail fast: wait until the server is healthy before touching the emulator.
	until curl -sf http://127.0.0.1:20000/health >/dev/null 2>&1; do
		if ! kill -0 $srv_pid 2>/dev/null; then
			echo "server failed to start — see the error above (config: workspace/{{conf}}, e.g. lib_root volume not mounted)"; exit 1
		fi
		sleep 1
	done
	# The ephemeral access token (regenerated every start) is only injected into the
	# server-served page; the APK shell never sees it, so surface it for the connect page.
	token=$(curl -s http://127.0.0.1:20000/ | sed -n 's/.*__PLAINSHELF_SECURITY__={token:"\([^"]*\)".*/\1/p')
	# Boot an emulator if none is running (emulator binary is not on PATH).
	if ! adb devices | grep -q '^emulator-.*device$'; then
		avd="${PLAINSHELF_AVD:-$("$ANDROID_HOME/emulator/emulator" -list-avds | head -n1)}"
		"$ANDROID_HOME/emulator/emulator" -avd "$avd" >/dev/null 2>&1 &
		adb wait-for-device
		until [[ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" == "1" ]]; do sleep 2; done
	fi
	target=$(adb devices | awk '$2 == "device" && /^emulator-/ { print $1; exit }')
	# cap run performs sync + gradle build + install + launch on the target.
	( cd {{srv_frontend_dir}} && npx cap run android --target "$target" )
	echo "Server running at 127.0.0.1:20000 — in the app connect to http://10.0.2.2:20000"
	echo "Access token for edits (paste in the app's connect page; changes every restart): ${token:-(security disabled)}"
	wait $srv_pid

# Package the macOS .app bundle as a zip suitable for the Homebrew cask
package-desktop-mac: build-desktop
	mkdir -p out
	ditto -c -k --sequesterRsrc --keepParent \
	  desktop/build/bin/PlainShelf.app \
	  out/plainshelf-desktop_{{version}}_darwin_arm64.zip
