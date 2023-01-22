package plugintypes

import (
	"io"
	"net/http"
)

// SetApp is used to allow GoBlog set its app instance to be accessible by the plugin.
type SetApp interface {
	SetApp(app App)
}

// SetConfig is used in all plugin types to allow GoBlog set the plugin configuration.
type SetConfig interface {
	SetConfig(config map[string]any)
}

type Exec interface {
	Exec()
}

type Middleware interface {
	Handler(next http.Handler) http.Handler
	Prio() int
}

type UI interface {
	Render(renderContext RenderContext, rendered io.Reader, modified io.Writer)
}
