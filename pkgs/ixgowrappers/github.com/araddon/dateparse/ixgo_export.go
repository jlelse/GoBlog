// export by github.com/goplus/ixgo/cmd/qexp

package dateparse

import (
	q "github.com/araddon/dateparse"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "dateparse",
		Path: "github.com/araddon/dateparse",
		Deps: map[string]string{
			"fmt":          "fmt",
			"strconv":      "strconv",
			"strings":      "strings",
			"time":         "time",
			"unicode":      "unicode",
			"unicode/utf8": "utf8",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{
			"ParserOption": reflect.TypeOf((*q.ParserOption)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars: map[string]reflect.Value{
			"ErrAmbiguousMMDD": reflect.ValueOf(&q.ErrAmbiguousMMDD),
		},
		Funcs: map[string]reflect.Value{
			"MustParse":                  reflect.ValueOf(q.MustParse),
			"ParseAny":                   reflect.ValueOf(q.ParseAny),
			"ParseFormat":                reflect.ValueOf(q.ParseFormat),
			"ParseIn":                    reflect.ValueOf(q.ParseIn),
			"ParseLocal":                 reflect.ValueOf(q.ParseLocal),
			"ParseStrict":                reflect.ValueOf(q.ParseStrict),
			"PreferMonthFirst":           reflect.ValueOf(q.PreferMonthFirst),
			"RetryAmbiguousDateWithSwap": reflect.ValueOf(q.RetryAmbiguousDateWithSwap),
		},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
