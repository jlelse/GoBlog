#!/bin/bash

GOFLAGS="-tags=linux,libsqlite3,sqlite_fts5" go get -d $(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -m all)
GOFLAGS="-tags=linux,libsqlite3,sqlite_fts5" go mod tidy -compat 1.18