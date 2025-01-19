package aibotblock

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	lastCached     time.Time
	botsCache      []string
	robotsTxtCache string
	mutex          sync.RWMutex
}

func GetPlugin() (
	plugintypes.SetApp,
	plugintypes.Middleware,
) {
	p := &plugin{
		botsCache:      []string{},
		robotsTxtCache: "",
		lastCached:     time.Now().AddDate(-1, 0, 0),
	}
	return p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.updateCache()
		if r.URL.Path == "/robots.txt" {
			p.mutex.RLock()
			w.Header().Set("cache-control", "no-cache, no-store, no-transform, must-revalidate, private, max-age=0")
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			_, _ = io.WriteString(w, p.robotsTxtCache)
			_, _ = io.Copy(w, rec.Result().Body)
			p.mutex.RUnlock()
			return
		} else if p.shouldBlock(r.Header.Get("User-Agent")) {
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
	return slices.ContainsFunc(p.botsCache, func(e string) bool {
		return strings.Contains(userAgent, e)
	})
}

func (p *plugin) updateCache() {
	if len(p.botsCache) == 0 || time.Since(p.lastCached) > 6*time.Hour {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		var resp map[string]any
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := requests.
			URL("https://raw.githubusercontent.com/ai-robots-txt/ai.robots.txt/refs/heads/main/robots.json").
			ToJSON(&resp).
			Fetch(timeoutCtx)
		if err == nil {
			newCache := []string{}
			newRobotsTxt := builderpool.Get()
			defer builderpool.Put(newRobotsTxt)
			for key := range resp {
				newCache = append(newCache, key)
				newRobotsTxt.WriteString("User-agent: ")
				newRobotsTxt.WriteString(key)
				newRobotsTxt.WriteString("\n")
			}
			newRobotsTxt.WriteString("Disallow: /\n\n")
			p.botsCache = newCache
			p.robotsTxtCache = newRobotsTxt.String()
			p.lastCached = time.Now()
		}
	}
}
