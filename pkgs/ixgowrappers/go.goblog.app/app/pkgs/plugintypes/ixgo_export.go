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
			"App":             reflect.TypeOf((*q.App)(nil)).Elem(),
			"BlockedBots":     reflect.TypeOf((*q.BlockedBots)(nil)).Elem(),
			"Blog":            reflect.TypeOf((*q.Blog)(nil)).Elem(),
			"Database":        reflect.TypeOf((*q.Database)(nil)).Elem(),
			"Exec":            reflect.TypeOf((*q.Exec)(nil)).Elem(),
			"Middleware":      reflect.TypeOf((*q.Middleware)(nil)).Elem(),
			"Post":            reflect.TypeOf((*q.Post)(nil)).Elem(),
			"PostCreatedHook": reflect.TypeOf((*q.PostCreatedHook)(nil)).Elem(),
			"PostDeletedHook": reflect.TypeOf((*q.PostDeletedHook)(nil)).Elem(),
			"PostUpdatedHook": reflect.TypeOf((*q.PostUpdatedHook)(nil)).Elem(),
			"RenderContext":   reflect.TypeOf((*q.RenderContext)(nil)).Elem(),
			"SetApp":          reflect.TypeOf((*q.SetApp)(nil)).Elem(),
			"SetConfig":       reflect.TypeOf((*q.SetConfig)(nil)).Elem(),
			"UI":              reflect.TypeOf((*q.UI)(nil)).Elem(),
			"UI2":             reflect.TypeOf((*q.UI2)(nil)).Elem(),
			"UIFooter":        reflect.TypeOf((*q.UIFooter)(nil)).Elem(),
			"UIImgAttributes": reflect.TypeOf((*q.UIImgAttributes)(nil)).Elem(),
			"UIPost":          reflect.TypeOf((*q.UIPost)(nil)).Elem(),
			"UIPostContent":   reflect.TypeOf((*q.UIPostContent)(nil)).Elem(),
			"UISummary":       reflect.TypeOf((*q.UISummary)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{
			"BlogStats":        reflect.TypeOf((*q.BlogStats)(nil)).Elem(),
			"BlogStatsRow":     reflect.TypeOf((*q.BlogStatsRow)(nil)).Elem(),
			"Comment":          reflect.TypeOf((*q.Comment)(nil)).Elem(),
			"CommentsQuery":    reflect.TypeOf((*q.CommentsQuery)(nil)).Elem(),
			"PostsQuery":       reflect.TypeOf((*q.PostsQuery)(nil)).Elem(),
			"Section":          reflect.TypeOf((*q.Section)(nil)).Elem(),
			"Webmention":       reflect.TypeOf((*q.Webmention)(nil)).Elem(),
			"WebmentionsQuery": reflect.TypeOf((*q.WebmentionsQuery)(nil)).Elem(),
		},
		AliasTypes:    map[string]reflect.Type{},
		Vars:          map[string]reflect.Value{},
		Funcs:         map[string]reflect.Value{},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
