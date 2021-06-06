package main

import "net/http"

const customPageContextKey = "custompage"

func (a *goBlog) serveCustomPage(w http.ResponseWriter, r *http.Request) {
	page := r.Context().Value(customPageContextKey).(*customPage)
	if a.cfg.Cache != nil && a.cfg.Cache.Enable && page.Cache {
		if page.CacheExpiration != 0 {
			setInternalCacheExpirationHeader(w, r, page.CacheExpiration)
		} else {
			setInternalCacheExpirationHeader(w, r, int(a.cfg.Cache.Expiration))
		}
	}
	a.render(w, r, page.Template, &renderData{
		BlogString: r.Context().Value(blogContextKey).(string),
		Canonical:  a.cfg.Server.PublicAddress + page.Path,
		Data:       page.Data,
	})
}
