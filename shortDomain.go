package main

import (
	"net/http"
)

func (a *goBlog) redirectShortDomain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if a.cfg.Server.shortPublicHostname != "" && r.Host == a.cfg.Server.shortPublicHostname {
			http.Redirect(rw, r, a.cfg.Server.PublicAddress+r.RequestURI, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(rw, r)
	})
}
