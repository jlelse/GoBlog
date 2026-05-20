#!/usr/bin/env bash
# Check direct go.mod dependencies for newer major versions
# (e.g., /v3 -> /v4, or unversioned -> /v2).
#
# Dependabot proposes these automatically, but this script is useful for
# quick manual audits.

set -euo pipefail

FLAGS="-tags=linux,libsqlite3,sqlite_fts5,gomailnotpl"

echo "Checking direct dependencies for newer major versions..."
echo

# Compute the candidate path for "current major + offset".
# offset=0 means the very next major after the current one.
next_path() {
  local path="$1" offset="$2"
  if [[ "$path" =~ ^(.+)/v([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]}/v$(( BASH_REMATCH[2] + 1 + offset ))"
  elif [[ "$path" =~ ^(gopkg\.in/.+)\.v([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]}.v$(( BASH_REMATCH[2] + 1 + offset ))"
  else
    echo "${path}/v$(( 2 + offset ))"
  fi
}

# Direct dependencies (path only)
deps=$(GOFLAGS=$FLAGS go list -m -f '{{if not (or .Indirect .Main)}}{{.Path}}{{end}}' all)

found=0
for path in $deps; do
  latest=""
  for offset in 0 1 2; do
    probe=$(next_path "$path" "$offset")
    output=$(GOFLAGS=$FLAGS go list -m -versions "$probe" 2>/dev/null || true)
    # Output is "<path> v1 v2 ..." if versions exist, just "<path>" otherwise.
    if [[ -n "$output" ]] && [[ "$(echo "$output" | awk '{print NF}')" -gt 1 ]]; then
      latest="$probe"
    else
      break
    fi
  done

  if [[ -n "$latest" ]]; then
    echo "  $path -> $latest"
    found=$(( found + 1 ))
  fi
done

echo
if [[ "$found" -eq 0 ]]; then
  echo "No newer major versions found."
else
  echo "Found $found dependency/dependencies with newer major versions."
fi
