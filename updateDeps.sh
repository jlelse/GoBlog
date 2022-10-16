#!/bin/bash

FLAGS="-tags=linux,libsqlite3,sqlite_fts5"
SKIP="
golang.org/x/crypto
"
EXTRA="
github.com/cretz/bine@master
github.com/tkrajina/gpxgo@master
github.com/yuin/goldmark-emoji@master
willnorris.com/go/microformats@main
"

checkSkip() { 
  echo $SKIP | grep -F -q -w "$1"
}

# Update all direct dependencies to latest version
echo "Check for updates..."

# 1. Get all direct dependency updates
updates=$(go list -f '{{if (and (not .Indirect) .Update)}}{{.Path}}{{end}}' -u -m all)

# 2. Update each dependency and tidy
for update in $updates
do
  if checkSkip $update
  then
    continue
  fi
  echo ""
  echo "Update $update"
  GOFLAGS=$FLAGS go get $update@latest
done

# 3. Update extra packages
for e in $EXTRA
do
  echo ""
  echo "Update $e"
  GOFLAGS=$FLAGS go get $e
done

# 4. Tidy
GOFLAGS=$FLAGS go mod tidy