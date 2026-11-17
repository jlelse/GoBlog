// export by github.com/goplus/ixgo/cmd/qexp

package bufferpool

import (
	q "go.goblog.app/app/pkgs/bufferpool"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "bufferpool",
		Path: "go.goblog.app/app/pkgs/bufferpool",
		Deps: map[string]string{
			"bytes": "bytes",
			"sync":  "sync",
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
