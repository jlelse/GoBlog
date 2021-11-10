package main

import (
	"encoding/json"
	"net/http"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	bc := a.cfg.Blogs[blog]

	allPostsWithLocation, err := a.getPosts(&postsRequestConfig{
		blog:               blog,
		status:             statusPublished,
		parameter:          a.cfg.Micropub.LocationParam,
		withOnlyParameters: []string{a.cfg.Micropub.LocationParam},
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
		if g := a.geoURI(p); g != nil {
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

	mapPath := bc.getRelativePath(defaultIfEmpty(bc.Map.Path, defaultGeoMapPath))
	a.render(w, r, templateGeoMap, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(mapPath),
		Data: map[string]interface{}{
			"locations": string(jb),
		},
	})
}
