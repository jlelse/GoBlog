// Package ixgowrappers registers the host packages available to plugins.
package ixgowrappers

import (
	// Register the Go standard library.
	_ "github.com/goplus/ixgo/pkg"
	// Register a larger reflectx icall pool for multiple plugin interpreters.
	_ "github.com/goplus/reflectx/icall/icall4096"

	// Register the packages that plugins may import.
	_ "go.goblog.app/app/pkgs/ixgowrappers/github.com/PuerkitoBio/goquery"
	_ "go.goblog.app/app/pkgs/ixgowrappers/github.com/araddon/dateparse"
	_ "go.goblog.app/app/pkgs/ixgowrappers/github.com/carlmjohnson/requests"
	_ "go.goblog.app/app/pkgs/ixgowrappers/go.goblog.app/app/pkgs/bufferpool"
	_ "go.goblog.app/app/pkgs/ixgowrappers/go.goblog.app/app/pkgs/builderpool"
	_ "go.goblog.app/app/pkgs/ixgowrappers/go.goblog.app/app/pkgs/htmlbuilder"
	_ "go.goblog.app/app/pkgs/ixgowrappers/go.goblog.app/app/pkgs/plugintypes"
	_ "go.goblog.app/app/pkgs/ixgowrappers/go.yaml.in/yaml/v4"
)

//go:generate go run github.com/goplus/ixgo/cmd/qexp -outdir . -filename ixgo_export -tags linux,libsqlite3,sqlite_fts5,gomailnotpl go.goblog.app/app/pkgs/plugintypes go.goblog.app/app/pkgs/htmlbuilder go.goblog.app/app/pkgs/bufferpool go.goblog.app/app/pkgs/builderpool github.com/PuerkitoBio/goquery github.com/carlmjohnson/requests github.com/araddon/dateparse go.yaml.in/yaml/v4
