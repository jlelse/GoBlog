// export by github.com/goplus/ixgo/cmd/qexp

package plugintypes

import (
	q "go.goblog.app/app/pkgs/plugintypes"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "plugintypes",
		Path: "go.goblog.app/app/pkgs/plugintypes",
		Deps: map[string]string{
			"context":                        "context",
			"database/sql":                   "sql",
			"github.com/PuerkitoBio/goquery": "goquery",
			"io":                             "io",
			"net/http":                       "http",
		},
		Interfaces: map[string]reflect.Type{
			"App":             reflect.TypeFor[q.App](),
			"BlockedBots":     reflect.TypeFor[q.BlockedBots](),
			"Blog":            reflect.TypeFor[q.Blog](),
			"Database":        reflect.TypeFor[q.Database](),
			"ErrorRenderer":   reflect.TypeFor[q.ErrorRenderer](),
			"Exec":            reflect.TypeFor[q.Exec](),
			"Middleware":      reflect.TypeFor[q.Middleware](),
			"Post":            reflect.TypeFor[q.Post](),
			"PostCreatedHook": reflect.TypeFor[q.PostCreatedHook](),
			"PostDeletedHook": reflect.TypeFor[q.PostDeletedHook](),
			"PostUpdatedHook": reflect.TypeFor[q.PostUpdatedHook](),
			"RenderContext":   reflect.TypeFor[q.RenderContext](),
			"Routes":          reflect.TypeFor[q.Routes](),
			"SetApp":          reflect.TypeFor[q.SetApp](),
			"SetConfig":       reflect.TypeFor[q.SetConfig](),
			"UI":              reflect.TypeFor[q.UI](),
			"UI2":             reflect.TypeFor[q.UI2](),
			"UIFooter":        reflect.TypeFor[q.UIFooter](),
			"UIImgAttributes": reflect.TypeFor[q.UIImgAttributes](),
			"UIPost":          reflect.TypeFor[q.UIPost](),
			"UIPostContent":   reflect.TypeFor[q.UIPostContent](),
			"UISummary":       reflect.TypeFor[q.UISummary](),
		},
		NamedTypes: map[string]reflect.Type{
			"BlogStats":        reflect.TypeFor[q.BlogStats](),
			"BlogStatsRow":     reflect.TypeFor[q.BlogStatsRow](),
			"Comment":          reflect.TypeFor[q.Comment](),
			"CommentsQuery":    reflect.TypeFor[q.CommentsQuery](),
			"PostsQuery":       reflect.TypeFor[q.PostsQuery](),
			"Route":            reflect.TypeFor[q.Route](),
			"Section":          reflect.TypeFor[q.Section](),
			"Webmention":       reflect.TypeFor[q.Webmention](),
			"WebmentionsQuery": reflect.TypeFor[q.WebmentionsQuery](),
		},
		AliasTypes:    map[string]reflect.Type{},
		Vars:          map[string]reflect.Value{},
		Funcs:         map[string]reflect.Value{},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
