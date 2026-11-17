// export by github.com/goplus/ixgo/cmd/qexp

package goquery

import (
	q "github.com/PuerkitoBio/goquery"

	"go/constant"
	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "goquery",
		Path: "github.com/PuerkitoBio/goquery",
		Deps: map[string]string{
			"errors":                          "errors",
			"github.com/andybalholm/cascadia": "cascadia",
			"golang.org/x/net/html":           "html",
			"io":                              "io",
			"iter":                            "iter",
			"net/http":                        "http",
			"net/url":                         "url",
			"strings":                         "strings",
		},
		Interfaces: map[string]reflect.Type{
			"Matcher": reflect.TypeFor[q.Matcher](),
		},
		NamedTypes: map[string]reflect.Type{
			"Document":    reflect.TypeFor[q.Document](),
			"Selection":   reflect.TypeFor[q.Selection](),
			"TextOptions": reflect.TypeFor[q.TextOptions](),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars:       map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"CloneDocument":           reflect.ValueOf(q.CloneDocument),
			"NewDocument":             reflect.ValueOf(q.NewDocument),
			"NewDocumentFromNode":     reflect.ValueOf(q.NewDocumentFromNode),
			"NewDocumentFromReader":   reflect.ValueOf(q.NewDocumentFromReader),
			"NewDocumentFromResponse": reflect.ValueOf(q.NewDocumentFromResponse),
			"NodeName":                reflect.ValueOf(q.NodeName),
			"OuterHtml":               reflect.ValueOf(q.OuterHtml),
			"Render":                  reflect.ValueOf(q.Render),
			"Single":                  reflect.ValueOf(q.Single),
			"SingleMatcher":           reflect.ValueOf(q.SingleMatcher),
			"Text":                    reflect.ValueOf(q.Text),
		},
		TypedConsts: map[string]ixgo.TypedConst{
			"ToEnd": {Typ: reflect.TypeOf(q.ToEnd), Value: constant.MakeInt64(int64(q.ToEnd))},
		},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
