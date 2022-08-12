package plugintypes

import (
	"net/http"

	"go.goblog.app/app/pkgs/htmlbuilder"
)

// SetApp is used in all plugin types to allow
// GoBlog set it's app instance to be accessible by the plugin.
type SetApp interface {
	SetApp(App)
}

// SetConfig is used in all plugin types to allow
// GoBlog set plugin configuration.
type SetConfig interface {
	SetConfig(map[string]any)
}

type Exec interface {
	SetApp
	SetConfig
	Exec()
}

type Middleware interface {
	SetApp
	SetConfig
	Handler(http.Handler) http.Handler
	Prio() int
}

type UI interface {
	SetApp
	SetConfig
	Render(*htmlbuilder.HtmlBuilder, RenderType, RenderData, RenderNextFunc)
}
