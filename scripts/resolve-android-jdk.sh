#!/usr/bin/env zsh

set -eu

is_java_21() {
    local candidate="$1"
    [[ -n "$candidate" && -x "$candidate/bin/java" ]] || return 1

    local version
    version="$("$candidate/bin/java" -XshowSettings:properties -version 2>&1 \
        | sed -n 's/^[[:space:]]*java\.specification\.version = //p' \
        | head -n 1)"
    [[ "$version" == "21" ]]
}

candidates=()
[[ -n "${JAVA_HOME:-}" ]] && candidates+=("$JAVA_HOME")
[[ -n "${STUDIO_JDK:-}" ]] && candidates+=("$STUDIO_JDK")

if [[ "$(uname -s)" == "Darwin" ]]; then
    registered_jdk="$(/usr/libexec/java_home -v 21 2>/dev/null || true)"
    [[ -n "$registered_jdk" ]] && candidates+=("$registered_jdk")
    candidates+=("/Applications/Android Studio.app/Contents/jbr/Contents/Home")
else
    candidates+=("/opt/android-studio/jbr" "/usr/local/android-studio/jbr")
fi

path_jdk="$(java -XshowSettings:properties -version 2>&1 \
    | sed -n 's/^[[:space:]]*java\.home = //p' \
    | head -n 1 || true)"
[[ -n "$path_jdk" ]] && candidates+=("$path_jdk")

for candidate in "${candidates[@]}"; do
    if is_java_21 "$candidate"; then
        print -r -- "$candidate"
        exit 0
    fi
done

print -u2 -- "JDK 21 is required for Capacitor 8 Android builds."
print -u2 -- "Install JDK 21 or set JAVA_HOME (or STUDIO_JDK) to a JDK 21 installation."
exit 1
