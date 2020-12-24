package main

import (
	"net/http"
)

func redirectShortDomain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if appConfig.Server.shortPublicHostname != "" && r.Host == appConfig.Server.shortPublicHostname {
			http.Redirect(rw, r, appConfig.Server.PublicAddress+r.RequestURI, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(rw, r)
	})
}
