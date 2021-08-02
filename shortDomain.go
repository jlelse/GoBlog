package main

import (
	"net/http"
)

func (a *goBlog) redirectShortDomain(rw http.ResponseWriter, r *http.Request) {
	http.Redirect(rw, r, a.getFullAddress(r.RequestURI), http.StatusMovedPermanently)
}
