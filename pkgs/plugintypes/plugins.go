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

// UISummary plugins get called when rendering the summary on indexes for a post.
type UISummary interface {
	// The renderContext provides information such as the path of the request or the blog name.
	// The post contains information about the post for which to render the summary.
	// The document can be used to add or modify the default HTML.
	RenderSummaryForPost(renderContext RenderContext, post Post, doc *goquery.Document)
}

// UIPost plugins get called when rendering the h-entry for a post. But only on the HTML frontend, not ActivityPub or feeds.
type UIPost interface {
	// The renderContext provides information such as the path of the request or the blog name.
	// The post contains information about the post for which to render the summary.
	// The document can be used to add or modify the default HTML. But it only contains the HTML for the post, not for the whole page.
	RenderPost(renderContext RenderContext, post Post, doc *goquery.Document)
}

// UIFooter plugins get called when rendering the footer on each HTML page.
type UIFooter interface {
	// The renderContext provides information such as the path of the request or the blog name.
	// The document can be used to add or modify the default HTML.
	RenderFooter(renderContext RenderContext, doc *goquery.Document)
}

// PostCreatedHook plugins get called after a post is created.
type PostCreatedHook interface {
	// Handle the post.
	PostCreated(post Post)
}

// PostUpdatedHook plugins get called after a post is updated.
type PostUpdatedHook interface {
	// Handle the post.
	PostUpdated(post Post)
}

// PostUpdatedHook plugins get called after a post is deleted.
type PostDeletedHook interface {
	// Handle the post.
	PostDeleted(post Post)
}
