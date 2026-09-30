#!/bin/sh

set -eu

usage() {
  echo "Usage: $0 <display|native|android-name|validate-tag|validate-next|release-branch|latest-tag> [version]" >&2
  exit 2
}

is_numeric_identifier() {
  case "$1" in
    ''|*[!0-9]*) return 1 ;;
    0) return 0 ;;
    0*) return 1 ;;
    *) return 0 ;;
  esac
}

is_core_version() {
  core=$1
  old_ifs=$IFS
  IFS=.
  set -- $core
  IFS=$old_ifs

  [ "$#" -eq 3 ] || return 1
  is_numeric_identifier "$1" &&
    is_numeric_identifier "$2" &&
    is_numeric_identifier "$3"
}

is_prerelease() {
  prerelease=$1
  [ -n "$prerelease" ] || return 1

  old_ifs=$IFS
  IFS=.
  set -- $prerelease
  IFS=$old_ifs

  for identifier do
    [ -n "$identifier" ] || return 1
    case "$identifier" in
      *[!0-9A-Za-z-]*) return 1 ;;
    esac
    case "$identifier" in
      *[!0-9]*) ;;
      *) is_numeric_identifier "$identifier" || return 1 ;;
    esac
  done
}

split_version() {
  candidate=$1
  case "$candidate" in
    v*) candidate=${candidate#v} ;;
  esac

  case "$candidate" in
    *+*) return 1 ;;
    *-*)
      core_version=${candidate%%-*}
      version_suffix=${candidate#*-}
      has_version_suffix=true
      ;;
    *)
      core_version=$candidate
      version_suffix=
      has_version_suffix=false
      ;;
  esac

  is_core_version "$core_version" || return 1
  if [ "$has_version_suffix" = true ]; then
    is_prerelease "$version_suffix" || return 1
  fi
}

is_release_tag() {
  case "$1" in
    v*) ;;
    *) return 1 ;;
  esac
  split_version "$1"
}

latest_release_tag() {
  for tag in $(git -c versionsort.suffix=- -c versionsort.suffix= tag --list 'v[0-9]*' --sort=-version:refname 2>/dev/null); do
    if is_release_tag "$tag"; then
      printf '%s\n' "$tag"
      return
    fi
  done
}

# Succeeds when SemVer identifier $1 sorts after $2.
identifier_gt() {
  if is_numeric_identifier "$1" && is_numeric_identifier "$2"; then
    [ "$1" -gt "$2" ]
  elif is_numeric_identifier "$1"; then
    return 1
  elif is_numeric_identifier "$2"; then
    return 0
  else
    [ "$1" != "$2" ] &&
      [ "$(printf '%s\n%s\n' "$1" "$2" | LC_ALL=C sort | tail -n 1)" = "$1" ]
  fi
}

# Succeeds when version $1 has higher SemVer precedence than $2.
version_gt() {
  split_version "$1" || return 1
  a_core=$core_version a_pre=$version_suffix a_has_pre=$has_version_suffix
  split_version "$2" || return 1
  b_core=$core_version b_pre=$version_suffix b_has_pre=$has_version_suffix

  while [ -n "$a_core" ]; do
    a_part=${a_core%%.*} b_part=${b_core%%.*}
    if [ "$a_part" -ne "$b_part" ]; then
      [ "$a_part" -gt "$b_part" ]
      return
    fi
    case "$a_core" in *.*) a_core=${a_core#*.} b_core=${b_core#*.} ;; *) a_core= ;; esac
  done

  # Equal cores: a release outranks any of its prereleases.
  [ "$a_has_pre" = "$b_has_pre" ] || { [ "$b_has_pre" = true ]; return; }
  [ "$a_has_pre" = true ] || return 1

  while :; do
    [ -n "$a_pre" ] || return 1
    [ -n "$b_pre" ] || return 0
    a_part=${a_pre%%.*} b_part=${b_pre%%.*}
    if [ "$a_part" != "$b_part" ]; then
      identifier_gt "$a_part" "$b_part"
      return
    fi
    case "$a_pre" in *.*) a_pre=${a_pre#*.} ;; *) a_pre= ;; esac
    case "$b_pre" in *.*) b_pre=${b_pre#*.} ;; *) b_pre= ;; esac
  done
}

resolve_display_version() {
  exact_tag=$(
    for tag in $(git -c versionsort.suffix=- -c versionsort.suffix= tag --points-at HEAD --sort=-version:refname 2>/dev/null); do
      if is_release_tag "$tag"; then
        printf '%s\n' "$tag"
        break
      fi
    done
  )
  if [ -n "$exact_tag" ]; then
    printf '%s\n' "$exact_tag"
    return
  fi

  latest_tag=$(latest_release_tag)

  short_commit=$(git rev-parse --short=7 HEAD 2>/dev/null || echo dev)
  dirty_suffix=
  if ! git diff-index --quiet HEAD -- 2>/dev/null; then
    dirty_suffix=-dirty
  fi

  if [ -z "$latest_tag" ]; then
    printf '%s%s\n' "$short_commit" "$dirty_suffix"
    return
  fi

  commit_count=$(git rev-list --count "$latest_tag"..HEAD 2>/dev/null || echo 0)
  printf '%s-%s-g%s%s\n' "$latest_tag" "$commit_count" "$short_commit" "$dirty_suffix"
}

[ "$#" -ge 1 ] && [ "$#" -le 2 ] || usage

mode=$1
if [ "$#" -eq 2 ]; then
  raw_version=$2
else
  raw_version=$(resolve_display_version)
fi

case "$mode" in
  display)
    printf '%s\n' "$raw_version"
    ;;
  native)
    if split_version "$raw_version"; then
      printf '%s\n' "$core_version"
    else
      printf '%s\n' '0.0.0'
    fi
    ;;
  android-name)
    if split_version "$raw_version"; then
      printf '%s\n' "${raw_version#v}"
    else
      printf '%s\n' '0.0.0-dev'
    fi
    ;;
  latest-tag)
    latest_release_tag
    ;;
  release-branch)
    if ! split_version "$raw_version"; then
      echo "Invalid version: $raw_version" >&2
      exit 1
    fi
    printf 'release/%s.x\n' "${core_version%%.*}"
    ;;
  validate-next)
    if ! is_release_tag "$raw_version"; then
      echo "Invalid release tag: $raw_version" >&2
      exit 1
    fi
    if git rev-parse -q --verify "refs/tags/$raw_version" >/dev/null; then
      echo "Tag already exists: $raw_version" >&2
      exit 1
    fi
    latest=$(latest_release_tag)
    if [ -n "$latest" ] && ! version_gt "$raw_version" "$latest"; then
      echo "Release tag $raw_version is not newer than $latest" >&2
      exit 1
    fi
    ;;
  validate-tag)
    if ! is_release_tag "$raw_version"; then
      echo "Invalid release tag: $raw_version" >&2
      exit 1
    fi
    ;;
  *)
    usage
    ;;
esac
