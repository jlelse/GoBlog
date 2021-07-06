package main

import (
	"encoding/json"
	"net/http"
)

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)

	allPostsWithLocation, err := a.db.getPosts(&postsRequestConfig{
		blog:               blog,
		status:             statusPublished,
		parameter:          geoParam,
		withOnlyParameters: []string{geoParam},
	})
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(allPostsWithLocation) == 0 {
		a.render(w, r, templateGeoMap, &renderData{
			BlogString: blog,
			Data: map[string]interface{}{
				"nolocations": true,
			},
		})
		return
	}

	type templateLocation struct {
		Lat  float64
		Lon  float64
		Post string
	}

	var locations []*templateLocation
	for _, p := range allPostsWithLocation {
		if g := p.GeoURI(); g != nil {
			locations = append(locations, &templateLocation{
				Lat:  g.Latitude,
				Lon:  g.Longitude,
				Post: p.Path,
			})
		}
	}

	jb, err := json.Marshal(locations)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	// Manipulate CSP header
	w.Header().Set(cspHeader, w.Header().Get(cspHeader)+" https://unpkg.com/ https://tile.openstreetmap.org")

	a.render(w, r, templateGeoMap, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"locations": string(jb),
		},
	})
}
