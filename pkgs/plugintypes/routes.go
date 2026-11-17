package plugintypes

import "net/http"

// Route describes an HTTP route provided by a plugin.
type Route struct {
	// Method is the HTTP method to match. An empty method matches all methods.
	Method string
	// Path is the route path, for example "/.well-known/example" or "/x/example".
	Path string
	// Blog restricts the route to a blog code. An empty blog registers a global route.
	Blog string
	// Handler handles requests matching the route.
	Handler http.Handler
}

// Routes plugins register HTTP routes.
type Routes interface {
	Routes() []Route
}
