// Package aibotblock provides an AI bot blocking plugin for GoBlog.
package aibotblock

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	lastCached time.Time
	botsCache  []string
	mutex      sync.RWMutex
}

// GetPlugin returns the aibotblock plugin instance.
func GetPlugin() (
	plugintypes.SetApp,
	plugintypes.Middleware,
	plugintypes.BlockedBots,
) {
	p := &plugin{
		botsCache:  []string{},
		lastCached: time.Now().AddDate(-2, 0, 0),
	}
	return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) BlockedBots() []string {
	p.updateCache()
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	result := make([]string, len(p.botsCache))
	copy(result, p.botsCache)
	return result
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.updateCache()
		if p.shouldBlock(r.Header.Get("User-Agent")) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (p *plugin) Prio() int {
	return 3000
}

func (p *plugin) shouldBlock(userAgent string) bool {
	if userAgent == "" {
		return false
	}
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	for _, bot := range p.botsCache {
		if strings.Contains(userAgent, bot) {
			return true
		}
	}
	return false
}

func (p *plugin) updateCache() {
	if len(p.botsCache) == 0 || time.Since(p.lastCached) > 24*time.Hour {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		var resp map[string]any
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := requests.
			URL("https://raw.githubusercontent.com/ai-robots-txt/ai.robots.txt/refs/heads/main/robots.json").
			ToJSON(&resp).
			Client(p.app.GetHTTPClient()).
			Fetch(timeoutCtx)
		if err == nil {
			newCache := make([]string, 0, len(resp))
			for key := range resp {
				newCache = append(newCache, key)
			}
			p.botsCache = newCache
			p.lastCached = time.Now()
		}
	}
}
