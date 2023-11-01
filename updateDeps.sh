#!/bin/bash

FLAGS="-tags=linux,libsqlite3,sqlite_fts5"
EXTRA="
github.com/cretz/bine@master
"

# Update all direct dependencies to latest version
echo "Check for updates..."

# 1. Update dependencies
GOFLAGS=$FLAGS go get -t -u ./...

# 2. Update extra packages
for e in $EXTRA
do
  echo ""
  echo "Update $e"
  GOFLAGS=$FLAGS go get $e
done

# 4. Tidy
GOFLAGS=$FLAGS go mod tidy