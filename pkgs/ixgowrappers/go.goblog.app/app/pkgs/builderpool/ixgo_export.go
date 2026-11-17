// export by github.com/goplus/ixgo/cmd/qexp

package builderpool

import (
	q "go.goblog.app/app/pkgs/builderpool"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "builderpool",
		Path: "go.goblog.app/app/pkgs/builderpool",
		Deps: map[string]string{
			"strings": "strings",
			"sync":    "sync",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{},
		AliasTypes: map[string]reflect.Type{},
		Vars:       map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"Get": reflect.ValueOf(q.Get),
			"Put": reflect.ValueOf(q.Put),
		},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
