package main

import "net/http"

const customPageContextKey = "custompage"

func serveCustomPage(w http.ResponseWriter, r *http.Request) {
	page := r.Context().Value(customPageContextKey).(*customPage)
	if appConfig.Cache != nil && appConfig.Cache.Enable && page.Cache {
		if page.CacheExpiration != 0 {
			setInternalCacheExpirationHeader(w, r, page.CacheExpiration)
		} else {
			setInternalCacheExpirationHeader(w, r, int(appConfig.Cache.Expiration))
		}
	}
	render(w, r, page.Template, &renderData{
		BlogString: r.Context().Value(blogContextKey).(string),
		Canonical:  appConfig.Server.PublicAddress + page.Path,
		Data:       page.Data,
	})
}
