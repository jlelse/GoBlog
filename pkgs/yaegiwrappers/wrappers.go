package yaegiwrappers

import (
	"reflect"
)

var (
	Symbols = make(map[string]map[string]reflect.Value)
)

// GoBlog packages
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers go.goblog.app/app/pkgs/plugintypes
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers go.goblog.app/app/pkgs/htmlbuilder
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers go.goblog.app/app/pkgs/bufferpool

// Dependencies
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers github.com/PuerkitoBio/goquery
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers github.com/carlmjohnson/requests
