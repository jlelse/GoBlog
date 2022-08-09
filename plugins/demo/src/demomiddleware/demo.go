package demomiddleware

import (
	"fmt"
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

func GetPlugin() plugintypes.Middleware {
	return &plugin{}
}

type plugin struct {
	app    plugintypes.App
	config map[string]any
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	p.config = config
}

func (p *plugin) Prio() int {
	if prioAny, ok := p.config["prio"]; ok {
		if prio, ok := prioAny.(int); ok {
			return prio
		}
	}
	return 100
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Demo", fmt.Sprintf("This is from the demo middleware with prio %d", p.Prio()))
		next.ServeHTTP(w, r)
	})
}
