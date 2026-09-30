#!/bin/sh

# Cuts CHANGELOG.md's [Unreleased] section into a release section, and prints a
# release section's body for the GitHub Release notes.

set -eu

usage() {
  echo "Usage: $0 cut <tag> <yyyy-mm-dd> [changelog]" >&2
  echo "       $0 notes <tag> [changelog]" >&2
  exit 2
}

has_section() {
  awk -v heading="## [$1]" '
    $0 == heading || index($0, heading " ") == 1 { found = 1 }
    END { exit !found }
  ' "$2"
}

cut_release() {
  tag=$1
  date=$2
  file=$3

  if has_section "$tag" "$file"; then
    echo "$file already has a [$tag] section" >&2
    exit 1
  fi
  if ! has_section Unreleased "$file"; then
    echo "$file has no [Unreleased] section" >&2
    exit 1
  fi

  tmp="$file.tmp.$$"
  # Moves [Unreleased] under the new heading, drops its empty subsections, and
  # leaves [Unreleased] with the empty headings update-changelog fills.
  if ! awk -v tag="$tag" -v date="$date" '
    function emit(    i, j, keep, released) {
      released = ""
      # Subsections start at "### "; one without a list entry is dropped.
      i = 1
      while (i <= n) {
        j = i + 1
        while (j <= n && line[j] !~ /^### /) j++
        keep = line[i] !~ /^### /
        for (k = i; k < j; k++) if (line[k] ~ /^[-*] /) { keep = 1; entries = 1 }
        if (keep) for (k = i; k < j; k++) released = released line[k] "\n"
        i = j
      }
      if (!entries) exit 3
      sub(/^\n+/, "", released)
      sub(/\n+$/, "\n", released)
      print "## [Unreleased]"
      print ""
      split("Added Changed Fixed Removed Security", heads, " ")
      for (h = 1; h <= 5; h++) { print "### " heads[h]; print "" }
      print "## [" tag "] - " date
      print ""
      printf "%s\n", released
      state = 2
    }
    state == 1 && /^## / { emit() }
    state == 1 { line[++n] = $0; next }
    state == 0 && $0 == "## [Unreleased]" { state = 1; next }
    { print }
    END { if (state == 1) emit() }
  ' "$file" >"$tmp"; then
    rm -f "$tmp"
    echo "$file has no entries under [Unreleased]" >&2
    exit 1
  fi
  mv "$tmp" "$file"
}

print_notes() {
  tag=$1
  file=$2

  # Body between the tag heading and the next level-2 heading, blank lines at
  # either end trimmed.
  notes=$(awk -v heading="## [$tag]" '
    in_section && /^## / { exit }
    in_section { print }
    $0 == heading || index($0, heading " ") == 1 { in_section = 1 }
  ' "$file" | sed -e '/./,$!d')
  if [ -z "$notes" ]; then
    echo "$file has no notes for [$tag]" >&2
    exit 1
  fi
  printf '%s\n' "$notes"
}

[ "$#" -ge 2 ] || usage

case "$1" in
  cut)
    [ "$#" -ge 3 ] && [ "$#" -le 4 ] || usage
    cut_release "$2" "$3" "${4:-CHANGELOG.md}"
    ;;
  notes)
    [ "$#" -le 3 ] || usage
    print_notes "$2" "${3:-CHANGELOG.md}"
    ;;
  *)
    usage
    ;;
esac
