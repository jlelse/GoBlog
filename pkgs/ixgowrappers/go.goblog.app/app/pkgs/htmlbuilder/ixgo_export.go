// export by github.com/goplus/ixgo/cmd/qexp

package htmlbuilder

import (
	q "go.goblog.app/app/pkgs/htmlbuilder"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "htmlbuilder",
		Path: "go.goblog.app/app/pkgs/htmlbuilder",
		Deps: map[string]string{
			"fmt":     "fmt",
			"io":      "io",
			"strings": "strings",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{
			"HTMLBuilder": reflect.TypeOf((*q.HTMLBuilder)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{
			"HtmlBuilder": reflect.TypeOf((*q.HtmlBuilder)(nil)).Elem(),
		},
		Vars: map[string]reflect.Value{
			"NewHtmlBuilder": reflect.ValueOf(&q.NewHtmlBuilder),
		},
		Funcs: map[string]reflect.Value{
			"NewHTMLBuilder": reflect.ValueOf(q.NewHTMLBuilder),
		},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
