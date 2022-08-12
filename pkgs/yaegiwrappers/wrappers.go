package yaegiwrappers

import (
	"reflect"
)

var (
	Symbols = make(map[string]map[string]reflect.Value)
)

//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers go.goblog.app/app/pkgs/plugintypes
//go:generate yaegi extract -license ../../LICENSE -name yaegiwrappers go.goblog.app/app/pkgs/htmlbuilder
