package plugintypes

import (
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// SetApp is used to allow GoBlog set its app instance to be accessible by the plugin.
type SetApp interface {
	SetApp(app App)
}

// SetConfig is used in all plugin types to allow GoBlog set the plugin configuration.
type SetConfig interface {
	SetConfig(config map[string]any)
}

// Exec plugins are executed after all plugins where initialized.
type Exec interface {
	// Exec gets called from a Goroutine, so it runs asynchronously.
	Exec()
}

// Middleware plugins can intercept and modify HTTP requests or responses.
type Middleware interface {
	Handler(next http.Handler) http.Handler
	// Return a priority, the higher prio middlewares get called first.
	Prio() int
}

// UI plugins get called when rendering HTML.
type UI interface {
	// rendered is a reader with all the rendered HTML, modify it and write it to modified. This is then returned to the client.
	// The renderContext provides information such as the path of the request or the blog name.
	Render(renderContext RenderContext, rendered io.Reader, modified io.Writer)
}

// UI2 plugins get called when rendering HTML.
type UI2 interface {
	// The renderContext provides information such as the path of the request or the blog name.
	// The document can be used to add or modify HTML.
	RenderWithDocument(renderContext RenderContext, doc *goquery.Document)
}
