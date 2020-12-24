package main

import "net/http"

func serveCustomPage(blog *configBlog, page *customPage) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		if appConfig.Cache != nil && appConfig.Cache.Enable && page.Cache {
			if page.CacheExpiration != 0 {
				setInternalCacheExpirationHeader(w, page.CacheExpiration)
			} else {
				setInternalCacheExpirationHeader(w, int(appConfig.Cache.Expiration))
			}
		}
		render(w, page.Template, &renderData{
			Blog:      blog,
			Canonical: appConfig.Server.PublicAddress + page.Path,
			Data:      page.Data,
		})
	}
}
