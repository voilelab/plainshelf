#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
changelog="$script_dir/release-changelog.sh"

work=$(mktemp -d "${TMPDIR:-/tmp}/plainshelf-changelog-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fail() {
  echo "$1" >&2
  exit 1
}

expect_file() {
  if ! diff -u "$2" "$1" >&2; then
    fail "$3"
  fi
}

cat >"$work/CHANGELOG.md" <<'EOF'
# Changelog

Intro.

## [Unreleased]

### Added

- Added one.
- Added two,
  wrapped.

### Changed

### Fixed

- Fixed three.

## [v0.1.0] - 2026-01-01

### Added

- Added zero.
EOF

cat >"$work/want.md" <<'EOF'
# Changelog

Intro.

## [Unreleased]

### Added

### Changed

### Fixed

### Removed

### Security

## [v0.2.0] - 2026-02-03

### Added

- Added one.
- Added two,
  wrapped.

### Fixed

- Fixed three.

## [v0.1.0] - 2026-01-01

### Added

- Added zero.
EOF

"$changelog" cut v0.2.0 2026-02-03 "$work/CHANGELOG.md"
expect_file "$work/CHANGELOG.md" "$work/want.md" "cut: unexpected CHANGELOG"

cat >"$work/want-notes.md" <<'EOF'
### Added

- Added one.
- Added two,
  wrapped.

### Fixed

- Fixed three.
EOF
"$changelog" notes v0.2.0 "$work/CHANGELOG.md" >"$work/notes.md"
expect_file "$work/notes.md" "$work/want-notes.md" "notes: unexpected body"

# The cut left only empty headings, so a second cut has nothing to release.
cp "$work/CHANGELOG.md" "$work/before.md"
if "$changelog" cut v0.3.0 2026-02-04 "$work/CHANGELOG.md" 2>/dev/null; then
  fail "cut accepted an empty [Unreleased]"
fi
expect_file "$work/CHANGELOG.md" "$work/before.md" "failed cut changed the file"

if "$changelog" cut v0.2.0 2026-02-04 "$work/CHANGELOG.md" 2>/dev/null; then
  fail "cut accepted an existing version"
fi
if "$changelog" notes v9.9.9 "$work/CHANGELOG.md" 2>/dev/null; then
  fail "notes accepted a missing version"
fi

# [Unreleased] as the last section, with entries outside any subsection.
printf '%s\n' '# Changelog' '' '## [Unreleased]' '' '- Loose entry.' >"$work/tail.md"
"$changelog" cut v1.0.0-rc1 2026-03-01 "$work/tail.md"
actual=$("$changelog" notes v1.0.0-rc1 "$work/tail.md")
[ "$actual" = "- Loose entry." ] || fail "notes at end of file: got '$actual'"
