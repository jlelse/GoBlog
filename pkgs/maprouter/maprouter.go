package maprouter

import (
	"net/http"
)

// Make sure interface is satisfied
var _ http.Handler = (*MapRouter)(nil)

// Routes requests based on a map with routers
type MapRouter struct {
	// Default http.Handler
	DefaultHandler http.Handler
	// Handlers mapped by prefix
	Handlers map[string]http.Handler
	// Optional function to find key for handler, default uses hostname
	KeyFunc func(r *http.Request) string
}

// Serve the HTTP request
func (ar *MapRouter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if len(ar.Handlers) > 0 {
		var key string
		if ar.KeyFunc != nil {
			key = ar.KeyFunc(r)
		} else {
			key = defaultKey(r)
		}
		if h, ok := ar.Handlers[key]; ok {
			h.ServeHTTP(rw, r)
			return
		}
	}
	ar.DefaultHandler.ServeHTTP(rw, r)
}

// Gets the default key for the router
func defaultKey(r *http.Request) string {
	return r.Host
}
